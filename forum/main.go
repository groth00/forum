package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	"github.com/groth00/forum/internal/mailer"
	"github.com/groth00/forum/internal/models"
	"github.com/joho/godotenv"
)

type config struct {
	host string
	port int
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	errorLog       *log.Logger
	infoLog        *log.Logger
	users          *models.UserModel
	topics         *models.TopicModel
	posts          *models.PostModel
	comments       *models.CommentModel
	tokens         *models.TokenModel
	templateCache  map[string]*template.Template
	formDecoder    *form.Decoder
	sessionManager *scs.SessionManager
	wg             sync.WaitGroup
	mailer         *mailer.Mailer
	config         config
}

func main() {
	var cfg config
	cfg.cors.trustedOrigins = []string{"http://localhost:4000"}

	flag.StringVar(&cfg.host, "host", "127.0.0.1", "interface to listen to")
	flag.IntVar(&cfg.port, "addr", 4000, "port to listen to")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "maximum open DB connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "maximum idle DB connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "max idle time before closing connections")
	flag.Func("cors-trusted-origins", "trusted origins, space separated", func(s string) error {
		cfg.cors.trustedOrigins = append(cfg.cors.trustedOrigins, strings.Fields(s)...)
		return nil
	})
	flag.Parse()

	_ = godotenv.Load(".env")
	cfg.db.dsn = os.Getenv("DSN")
	cfg.smtp.host = os.Getenv("SMTP_HOST")
	cfg.smtp.username = os.Getenv("SMTP_USERNAME")
	cfg.smtp.password = os.Getenv("SMTP_PASSWORD")
	cfg.smtp.sender = os.Getenv("SMTP_SENDER")

	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

	db, err := openDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	templateCache, err := newTemplateCache()
	if err != nil {
		errorLog.Fatal(err)
	}

	formDecoder := form.NewDecoder()

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Secure = true

	mailClient, err := mailer.New(cfg.smtp.host, cfg.smtp.username, cfg.smtp.password)
	if err != nil {
		errorLog.Fatal(err)
	}

	app := &application{
		errorLog:       errorLog,
		infoLog:        infoLog,
		users:          &models.UserModel{DB: db},
		topics:         &models.TopicModel{DB: db},
		posts:          &models.PostModel{DB: db},
		comments:       &models.CommentModel{DB: db},
		tokens:         &models.TokenModel{DB: db},
		templateCache:  templateCache,
		formDecoder:    formDecoder,
		sessionManager: sessionManager,
		config:         cfg,
		mailer:         mailClient,
	}

	err = app.serve()
	errorLog.Println(err)
}

func (app *application) serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	otelShutdown, err := setupOtelSDK(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	serverError := make(chan error, 1)
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", app.config.host, app.config.port),
		ErrorLog:     app.errorLog,
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		app.infoLog.Printf("Starting server on %s:%d", app.config.host, app.config.port)
		serverError <- srv.ListenAndServe()
	}()

	select {
	case err = <-serverError:
		return err
	case <-ctx.Done():
		stop()
		app.infoLog.Printf("Waiting for in-flight requests to complete.")
		app.wg.Wait()
	}

	app.infoLog.Printf("Shutting down server on %s:%d.", app.config.host, app.config.port)
	return srv.Shutdown(ctx)
}

func (app *application) background(fn func()) {
	app.wg.Add(1)

	go func() {
		defer app.wg.Done()

		defer func() {
			if err := recover(); err != nil {
				app.errorLog.Println(fmt.Errorf("%s", err))
			}
		}()

		fn()
	}()
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}
