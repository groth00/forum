package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/groth00/forum/internal/models"
)

type topicCreateForm struct {
	Name      string `form:"name"`
	Validator `form:"-"`
}

type topicUpdateForm struct {
	Name      string `form:"name"`
	Validator `form:"-"`
}

func (app *application) topicGet(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	topic, err := app.topics.Get(topic_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	posts, err := app.posts.GetByTopic(topic_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	data := app.newTemplateData(r)
	data.Topic = topic
	data.Posts = posts
	app.render(w, http.StatusOK, "topic.tmpl", data)
}

func (app *application) topicList(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()

	var limit int
	if raw := qp.Get("limit"); raw != "" {
		if temp, err := strconv.Atoi(raw); err != nil {
			app.serverError(w, err)
			return
		} else {
			limit = temp
		}
	} else {
		limit = 10
	}

	topics, err := app.topics.List(limit)
	if err != nil {
		app.serverError(w, err)
		return
	}

	data := app.newTemplateData(r)
	data.Topics = topics
	app.render(w, http.StatusOK, "topics.tmpl", data)
}

func (app *application) topicCreate(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &topicCreateForm{}
	app.render(w, http.StatusOK, "topic_create.tmpl", data)
}

func (app *application) topicCreatePost(w http.ResponseWriter, r *http.Request) {
	var form topicCreateForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.Name), "name", "topic name cannot be blank")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "topic_create.tmpl", data)
		return
	}

	id, err := app.topics.Insert(form.Name)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "flash", "Topic successfully created!")
	http.Redirect(w, r, fmt.Sprintf("/topics/%d", id), http.StatusSeeOther)
}

func (app *application) topicUpdatePost(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	topic, err := app.topics.Get(topic_id)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			app.notFound(w, r)
		default:
			app.serverError(w, err)
			return
		}
	}

	var form topicUpdateForm
	err = app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.Name), "name", "topic name cannot be blank")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "topic_update.tmpl", data)
		return
	}

	topic.Name = form.Name
	err = app.topics.Update(topic)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "flash", "Topic successfully updated!")
}

func (app *application) topicDelete(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.topics.Delete(topic_id)
	if err != nil {
		app.serverError(w, err)
	}
}

func (app *application) topicAddModerator(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	user_id, err := app.getQueryParameterInt(w, r, "user_id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	user, err := app.users.Get(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.topics.AddModerator(topic_id, user_id, user.Name)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) topicRemoveModerator(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	user_id, err := app.getQueryParameterInt(w, r, "user_id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.topics.RemoveModerator(topic_id, user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) topicSubscribe(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	if user_id == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = app.topics.Subscribe(topic_id, user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) topicUnsubscribe(w http.ResponseWriter, r *http.Request) {
	topic_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	if user_id == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = app.topics.Unsubscribe(topic_id, user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}
