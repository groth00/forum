package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/groth00/forum/internal/models"
	"go.opentelemetry.io/otel/attribute"
)

type commentCreateForm struct {
	PostID    int    `form:"post_id"`
	ParentID  int    `form:"parent_id"`
	Content   string `form:"content"`
	Validator `form:"-"`
}

type commentUpdateForm struct {
	UserID  int    `form:"user_id"`
	Content string `form:"content"`
}

func (app *application) commentGet(w http.ResponseWriter, r *http.Request) {
	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	comment, err := app.comments.Get(comment_id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecordFound) {
			app.notFound(w, r)
			return
		} else {
			app.serverError(w, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Comment = comment
	app.render(w, http.StatusOK, "comment.tmpl", data)
}

func (app *application) commentCreatePost(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "commentCreate")
	defer span.End()

	var form commentCreateForm

	span.AddEvent("Decoding form")
	err := app.decodePostForm(r, &form)
	if err != nil {
		app.errorLog.Println("failed to decode form")
		app.clientError(w, http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("form_content", form.Content),
		attribute.Int("post_id", form.PostID),
		attribute.Int("parent_id", form.ParentID),
	)

	span.AddEvent("Validating user input")
	form.CheckField(NotBlank(form.Content), "content", "content cannot be blank")
	form.CheckField(MaxChars(form.Content, 2048), "content", "can be at most 2048 characters")

	span.AddEvent("Checking for valid post ID")
	post, err := app.posts.Get(form.PostID)
	if err != nil {
		app.serverError(w, err)
		return
	}
	if post == nil {
		form.AddNonFieldError("tried to create a comment for a non-existent post")
	}

	span.AddEvent("Checking for valid parent ID if specified")
	if form.ParentID > 0 {
		_, err = app.comments.Get(form.ParentID)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrNoRecordFound):
				form.AddNonFieldError("tried to reply to a non-existent parent comment")
			default:
				app.serverError(w, err)
				return
			}
		}
	}

	span.AddEvent("Inserting comment into database")
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	user, err := app.users.Get(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	_, err = app.comments.Insert(user_id, form.PostID, form.ParentID, user.Name, form.Content)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrStartTransaction), errors.Is(err, models.ErrCommitTransaction):
			app.errorLog.Println("failed to start/commit transaction")
			return
		case errors.Is(err, models.ErrInsertComment):
			app.errorLog.Println("failed to insert new comment")
			return
		case errors.Is(err, models.ErrInsertCommentPath):
			app.errorLog.Println("failed to insert ancestor/descendant paths")
			return
		case errors.Is(err, models.ErrInsertCommentLike):
			app.errorLog.Println("failed to add comment like")
			return
		case errors.Is(err, models.ErrUpdatePostCommentCount):
			app.errorLog.Println("failed to increment number of comments")
			return
		default:
			app.serverError(w, err)
			return
		}
	}

	app.sessionManager.Put(r.Context(), "flash", "Comment created!")
	http.Redirect(w, r, fmt.Sprintf("/posts/%d", form.PostID), http.StatusSeeOther)
}

func (app *application) commentDelete(w http.ResponseWriter, r *http.Request) {
	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.serverError(w, err)
		return
	}

	comment, err := app.comments.Get(comment_id)
	if err != nil {
		app.clientError(w, http.StatusNotFound)
		return
	}

	if comment.UserID != app.sessionManager.GetInt(r.Context(), "authenticatedUserID") {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = app.comments.Delete(comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) commentUpdatePost(w http.ResponseWriter, r *http.Request) {
	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	comment, err := app.comments.Get(comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if comment.UserID != app.sessionManager.GetInt(r.Context(), "authenticatedUserID") {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	var form commentUpdateForm
	err = app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	validator := Validator{}
	validator.CheckField(NotBlank(form.Content), "content", "content cannot be blank")
	validator.CheckField(MaxChars(form.Content, 2048), "content", "content can be at most 2048 characters")

	if !validator.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "comment_update.tmpl", data)
		return
	}

	comment.Content = form.Content
	err = app.comments.Update(comment)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "flash", "Comment successfully updated!")
	http.Redirect(w, r, fmt.Sprintf("/comments/%d", comment.ID), http.StatusSeeOther)
}

func (app *application) commentLike(w http.ResponseWriter, r *http.Request) {
	_, trace := tracer.Start(r.Context(), "comment_like")
	defer trace.End()

	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	trace.SetAttributes(
		attribute.Int("comment_id", comment_id),
		attribute.Int("user_id", user_id),
	)

	trace.AddEvent("Adding comment like to table")
	err = app.comments.Like(user_id, comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) commentDislike(w http.ResponseWriter, r *http.Request) {
	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.comments.Dislike(user_id, comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) commentSave(w http.ResponseWriter, r *http.Request) {
	_, trace := tracer.Start(r.Context(), "comment_save")
	defer trace.End()

	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	trace.SetAttributes(
		attribute.Int("comment_id", comment_id),
		attribute.Int("user_id", user_id),
	)

	trace.AddEvent("Saving comment to table")
	err = app.comments.Save(user_id, comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) commentUnsave(w http.ResponseWriter, r *http.Request) {
	comment_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	err = app.comments.Unsave(user_id, comment_id)
	if err != nil {
		app.serverError(w, err)
		return
	}
}
