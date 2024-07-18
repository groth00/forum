package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/groth00/forum/internal/models"
)

type userSigninForm struct {
	Email    string `form:"email"`
	Password string `form:"password"`
	Validator
}

type userRegisterForm struct {
	Name     string `form:"name"`
	Email    string `form:"email"`
	Password string `form:"password"`
	Validator
}

type userActivateForm struct {
	Token     string `form:"token"`
	Validator `form:"-"`
}

type userPasswordResetForm struct {
	New       string `form:"password"`
	Confirm   string `form:"confirm"`
	Validator `form:"-"`
}

func (app *application) userGet(w http.ResponseWriter, r *http.Request) {
	user_id, err := app.getIDParam(w, r, "id")
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	data := app.newTemplateData(r)
	user, err := app.users.Get(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	data.User = user
	app.render(w, http.StatusOK, "user.tmpl", data)
}

func (app *application) userList(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	users, err := app.users.List()
	if err != nil {
		app.serverError(w, err)
		return
	}
	data.Users = users
	app.render(w, http.StatusOK, "home.tmpl", data)
}

func (app *application) userCommentSaved(w http.ResponseWriter, r *http.Request) {
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	comments, err := app.users.GetSavedComments(user_id)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			break
		default:
			app.serverError(w, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Comments = comments
	app.render(w, http.StatusOK, "comments_saved.tmpl", data)
}

func (app *application) userPostSaved(w http.ResponseWriter, r *http.Request) {
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	posts, err := app.users.GetSavedPosts(user_id)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			break
		default:
			app.serverError(w, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Posts = posts
	app.render(w, http.StatusOK, "posts_saved.tmpl", data)
}

func (app *application) userCommentLiked(w http.ResponseWriter, r *http.Request) {
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	comments, err := app.users.GetLikedComments(user_id)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			break
		default:
			app.serverError(w, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Comments = comments
	app.render(w, http.StatusOK, "comments_liked.tmpl", data)
}

func (app *application) userPostLiked(w http.ResponseWriter, r *http.Request) {
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	posts, err := app.users.GetLikedPosts(user_id)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			break
		default:
			app.serverError(w, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Posts = posts
	app.render(w, http.StatusOK, "posts_liked.tmpl", data)
}

func (app *application) userLogin(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &userSigninForm{}
	app.render(w, http.StatusOK, "login.tmpl", data)
}

func (app *application) userLoginPost(w http.ResponseWriter, r *http.Request) {
	var form userSigninForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.Email), "email", "email cannot be blank")
	form.CheckField(Matches(form.Email, EmailRegex), "email", "invalid email format")
	form.CheckField(NotBlank(form.Password), "password", "password cannot be blank")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "login.tmpl", data)
		return
	}

	user_id, err := app.users.Authenticate(form.Email, form.Password)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			form.AddNonFieldError("invalid email or password")
			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "login.tmpl", data)
			return
		} else {
			app.serverError(w, err)
		}
		return
	}

	err = app.sessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", user_id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) userLogoutPost(w http.ResponseWriter, r *http.Request) {
	err := app.sessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Remove(r.Context(), "authenticatedUserID")
	app.sessionManager.Put(r.Context(), "flash", "You have logged out.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) userCreate(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &userRegisterForm{}
	app.render(w, http.StatusOK, "register.tmpl", data)
}

func (app *application) userCreatePost(w http.ResponseWriter, r *http.Request) {
	var form userRegisterForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.Name), "name", "username cannot be blank")
	form.CheckField(MaxChars(form.Name, 500), "name", "cannot be longer than 500 bytes")

	form.CheckField(NotBlank(form.Email), "email", "email cannot be blank")
	form.CheckField(Matches(form.Email, EmailRegex), "email", "must be a valid email")

	form.CheckField(NotBlank(form.Password), "password", "password cannot be blank")
	form.CheckField(MinChars(form.Password, 8), "password", "cannot be shorter than 8 bytes")
	form.CheckField(MaxChars(form.Password, 72), "password", "cannot be longer than 72 bytes")

	if !form.Valid() {
		data := app.newTemplateData(r)
		form.Password = ""
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "register.tmpl", data)
		return
	}

	user_id, err := app.users.Insert(form.Name, form.Email, form.Password)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrDuplicateEmail):
			form.AddFieldError("email", "email is already in use")
			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "register.tmpl", data)
			return
		case errors.Is(err, models.ErrDuplicateUsername):
			form.AddFieldError("name", "username is already in use")
			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "register.tmpl", data)
			return
		default:
			app.serverError(w, err)
			return
		}
	}

	token, err := app.tokens.New(user_id, time.Hour*24, models.ScopeActivation)
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.tokens.Insert(token)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user_id,
		}

		err := app.mailer.Send(app.config.smtp.sender, form.Email, "register_email.tmpl", data)
		if err != nil {
			app.errorLog.Printf("Error sending mail: %v", err)
		}
	})

	app.sessionManager.Put(r.Context(), "flash", "User was successfully created!")
	http.Redirect(w, r, "/users/activate", http.StatusSeeOther)
}

func (app *application) userActivate(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &userActivateForm{}
	app.render(w, http.StatusOK, "activate.tmpl", data)
}

func (app *application) userActivatePost(w http.ResponseWriter, r *http.Request) {
	var form userActivateForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.Token), "token", "token cannot be blank")
	form.CheckField(len(form.Token) == 52, "token", "token length must be 52 bytes")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusUnprocessableEntity, "activate.tmpl", data)
		return
	}

	user, err := app.users.GetByToken(form.Token, models.ScopeActivation)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoRecordFound):
			form.AddFieldError("token", "invalid token")
			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "activate.tmpl", data)
			return
		default:
			app.serverError(w, err)
			return
		}
	}

	user.Activated = true
	err = app.users.Update(user)
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.tokens.DeleteAllForUser(user.ID, models.ScopeActivation)
	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", user.ID)
	app.sessionManager.Put(r.Context(), "flash", "Your account has been successfully activated!")
	http.Redirect(w, r, "/users/login", http.StatusSeeOther)
}

func (app *application) userDelete(w http.ResponseWriter, r *http.Request) {
	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")

	if user_id == 1 {
		app.infoLog.Println("cannot delete admin user")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	err := app.users.Delete(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) userSettings(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &userPasswordResetForm{}
	app.render(w, http.StatusOK, "user_settings.tmpl", data)
}

func (app *application) userPasswordResetPost(w http.ResponseWriter, r *http.Request) {
	var form userPasswordResetForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(NotBlank(form.New), "password", "new password cannot be blank")
	form.CheckField(MinChars(form.New, 8), "password", "new password must be at least 8 bytes")
	form.CheckField(NotBlank(form.Confirm), "confirm", "confirmation password cannot be blank")

	if form.New != form.Confirm {
		form.AddNonFieldError("passwords do not match")
	}

	user_id := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	user, err := app.users.Get(user_id)
	if err != nil {
		app.serverError(w, err)
		return
	}

	err = app.users.UpdatePassword(user)
	if err != nil {
		app.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/users/settings", http.StatusSeeOther)
}
