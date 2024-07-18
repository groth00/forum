package main

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/otel"
)

const otelName = "forum"

var (
	tracer = otel.GetTracerProvider().Tracer(otelName)
	meter  = otel.GetMeterProvider().Meter(otelName)
)

// do not delete
func init() {}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "home")
	defer span.End()

	topics, err := app.topics.List(10)
	if err != nil {
		app.serverError(w, err)
		return
	}

	data := app.newTemplateData(r)
	data.Topics = topics
	app.render(w, http.StatusOK, "home.tmpl", data)
}

func (app *application) ping(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "ping")
	defer span.End()
	w.Write([]byte("pong"))
}

func (app *application) getIDParam(w http.ResponseWriter, r *http.Request, key string) (int, error) {
	params := httprouter.ParamsFromContext(r.Context())
	temp := params.ByName(key)
	id, err := strconv.Atoi(temp)
	if err != nil {
		app.serverError(w, errors.New("failed to convert ID param into int"))
		return -1, err
	}
	return id, nil
}

func (app *application) getQueryParameter(w http.ResponseWriter, r *http.Request, key string) (string, error) {
	qp := r.URL.Query()
	if temp := qp.Get(key); temp != "" {
		return temp, nil
	}
	return "", errors.New(fmt.Errorf("key %s not found in query parameters", key).Error())
}

func (app *application) getQueryParameterWithDefault(w http.ResponseWriter, r *http.Request, key, defaultValue string) string {
	qp := r.URL.Query()
	if value := qp.Get(key); value != "" {
		return value
	} else {
		return defaultValue
	}
}

func (app *application) getQueryParameterInt(w http.ResponseWriter, r *http.Request, key string) (int, error) {
	qp := r.URL.Query()
	if temp := qp.Get(key); temp != "" {
		converted, err := strconv.Atoi(temp)
		if err != nil {
			return -1, err
		}
		return converted, nil
	}
	return -1, errors.New(fmt.Errorf("key %s not found in query parameters", key).Error())
}

func (app *application) serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *application) notFound(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
