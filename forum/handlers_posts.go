package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/groth00/forum/internal/models"
)

const (
	sortMethodLikes = "likes"
	sortMethodNew   = "new"
	sortMethodOld   = "old"
)

type postCreateForm struct {
	TopicID   int    `form:"topic_id"`
	Title     string `form:"title"`
	Content   string `form:"content"`
	Validator `form:"-"`
}

type postUpdateForm struct {
	PostID  int    `form:"post_id"`
	Title   string `form:"title"`
	Content string `form:"content"`
}

func (app *application) postGet(w http.ResponseWriter, r *http.Request) {
	id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.infoLog.Println("client error, bad ID")
		app.clientError(w, http.StatusBadRequest)
		return
	}

	sortMethod := app.getQueryParameterWithDefault(w, r, "sortMethod", sortMethodLikes)

	post, err := app.posts.Get(id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	comments, err := app.comments.GetForPost(post.ID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoCommentsForPost):
			break
		default:
			app.serverError(w, err)
			return
		}
	}

	if comments != nil {
		switch sortMethod {
		case sortMethodOld:
			slices.SortFunc(comments, func(a, b *models.CommentNode) int {
				return int(b.Created.Unix() - a.Created.Unix())
			})
		case sortMethodNew:
			slices.SortFunc(comments, func(a, b *models.CommentNode) int {
				return int(a.Created.Unix() - b.Created.Unix())
			})
		case sortMethodLikes:
			slices.SortFunc(comments, func(a, b *models.CommentNode) int {
				return b.Likes - a.Likes
			})
		}
	}

	data := app.newTemplateData(r)
	data.Form = &commentCreateForm{}
	data.Post = post
	data.CommentNodes = comments
	app.render(w, http.StatusOK, "post.tmpl", data)
}

func (app *application) postList(w http.ResponseWriter, r *http.Request) {
	var limit int

	qp := r.URL.Query()
	if temp := qp.Get("limit"); temp != "" {
		converted, err := strconv.Atoi(temp)
		if err != nil {
			app.clientError(w, http.StatusBadRequest)
			return
		} else {
			limit = converted
		}
	}

	posts, err := app.posts.List(limit)
	if err != nil {
		app.serverError(w, err)
		return
	}

	data := app.newTemplateData(r)
	data.Posts = posts
	app.render(w, http.StatusOK, "posts.tmpl", data)
}

func (app *application) postCreate(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &postCreateForm{}
	app.render(w, http.StatusOK, "post_create.tmpl", data)
}

// TODO: frontend autofills the id if creating within a topic, otherwise it must be provided
func (app *application) postCreatePost(w http.ResponseWriter, r *http.Request) {
	var form postCreateForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(ValidInt(form.TopicID), "topic", "must be an integer")
	form.CheckField(NotBlank(form.Title), "title", "title cannot be blank")
	form.CheckField(MaxChars(form.Title, 64), "title", "title can be at most 64 characters")
	form.CheckField(NotBlank(form.Content), "content", "content cannot be blank")
	form.CheckField(MaxChars(form.Content, 2048), "content", "content can be at most 2048 characters")

	_, err = app.topics.Get(form.TopicID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			form.AddNonFieldError("topic ID does not exist")
		default:
			app.serverError(w, err)
			return
		}
	}

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "post_create.tmpl", data)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	user, err := app.users.Get(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	post_id, err := app.posts.Insert(user.ID, form.TopicID, user.Name, form.Title, form.Content)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "flash", "Post successfully created!")
	http.Redirect(w, r, fmt.Sprintf("/posts/%d", post_id), http.StatusSeeOther)
}

func (app *application) postDelete(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	post, err := app.posts.Get(post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if post.UserID != app.sessionManager.GetInt(r.Context(), "authenticatedUserID") {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = app.posts.Delete(post.UserID, post.ID, post.TopicID)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) postUpdate(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	post, err := app.posts.Get(post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if post.UserID != app.sessionManager.GetInt(r.Context(), "authenticatedUserID") {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	data := app.newTemplateData(r)
	data.Form = &postUpdateForm{post.ID, post.Title, post.Content}
	app.render(w, http.StatusOK, "post_update.tmpl", data)
}

func (app *application) postUpdatePost(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	post, err := app.posts.Get(post_id)
	if err != nil {
		app.clientError(w, http.StatusNotFound)
		return
	}

	if post.UserID != app.sessionManager.GetInt(r.Context(), "authenticatedUserID") {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	var form postUpdateForm
	err = app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	validator := Validator{}
	validator.CheckField(NotBlank(form.Title), "title", "title cannot be blank")
	validator.CheckField(MaxChars(form.Title, 64), "title", "title can be at most 64 characters")
	validator.CheckField(NotBlank(form.Content), "content", "content cannot be blank")
	validator.CheckField(MaxChars(form.Content, 2048), "content", "content can be at most 2048 characters")

	if !validator.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "post_update.tmpl", data)
		return
	}

	post.Title = form.Title
	post.Content = form.Content
	err = app.posts.Update(post)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "flash", "Post successfully updated!")
	http.Redirect(w, r, fmt.Sprintf("/posts/%d", post_id), http.StatusSeeOther)
}

func (app *application) postLike(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.posts.Like(user_id, post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) postDislike(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.posts.Dislike(user_id, post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) postSave(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.posts.Save(user_id, post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) postUnsave(w http.ResponseWriter, r *http.Request) {
	post_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.posts.Unsave(user_id, post_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}
