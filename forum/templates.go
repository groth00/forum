package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"github.com/groth00/forum/internal/models"
	"github.com/groth00/forum/ui"
	"github.com/justinas/nosurf"
)

type templateData struct {
	CurrentYear     int
	User            *models.User
	Users           []*models.User
	Topic           *models.Topic
	Topics          []*models.Topic
	Post            *models.Post
	Posts           []*models.Post
	Comment         *models.Comment
	Comments        []*models.Comment
	CommentNodes    []*models.CommentNode
	Form            any
	Flash           string
	IsAuthenticated bool
	CSRFToken       string
}

func formatDate(in time.Time) string {
	return in.UTC().Format(time.DateTime)
}

func setCommentMargin(path_length int) int {
	return 30 * path_length
}

func varargs(args ...interface{}) []interface{} {
	return args
}

func (app *application) newTemplateData(r *http.Request) *templateData {
	return &templateData{
		CurrentYear:     time.Now().Year(),
		Flash:           app.sessionManager.PopString(r.Context(), "flash"),
		IsAuthenticated: app.isAuthenticated(r),
		CSRFToken:       nosurf.Token(r),
	}
}

func (app *application) render(w http.ResponseWriter, status int, page string, data *templateData) {
	ts, ok := app.templateCache[page]
	if !ok {
		err := fmt.Errorf("the template %s does not exist", page)
		app.serverError(w, err)
	}

	buf := new(bytes.Buffer)

	// write template to buffer to catch any error before presenting to users
	err := ts.ExecuteTemplate(buf, "base", data)
	if err != nil {
		app.serverError(w, err)
		return
	}

	w.WriteHeader(status)
	buf.WriteTo(w) // write from bytes.buffer to http.ResponseWriter
}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	templateFuncs := map[string]any{
		"formatDate":       formatDate,
		"setCommentMargin": setCommentMargin,
		"varargs":          varargs,
	}

	pages, err := fs.Glob(ui.Files, "html/templates/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		patterns := []string{
			"html/base.tmpl",
			"html/nav.tmpl",
			"html/aside.tmpl",
			page,
		}
		// parse templates from the embedded filesystem
		ts, err := template.New(name).Funcs(templateFuncs).ParseFS(ui.Files, patterns...)
		if err != nil {
			return nil, err
		}

		// add full page to cache
		cache[name] = ts
	}
	return cache, nil
}
