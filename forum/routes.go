package main

import (
	"net/http"

	"github.com/groth00/forum/ui"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(app.notFound)

	// serve static assets from embedded FS
	fileServer := http.FileServer(http.FS(ui.Files))
	router.Handler(http.MethodGet, "/static/*filepath", fileServer)

	// middleware chains
	// removing noSurf because it's not possible to store the CSRFToken in the recursive HTML
	session := alice.New(app.sessionManager.LoadAndSave, noSurf, app.authenticate)
	authenticated := session.Append(app.requireAuthentication)
	activated := authenticated.Append(app.requireActivatedUser)
	admin := activated.Append(app.requireAdmin)
	middle := alice.New(app.recoverPanic, app.enableCORS, app.logRequest, secureHeaders)

	home := otelhttp.WithRouteTag("/", session.ThenFunc(app.home))
	router.Handler(http.MethodGet, "/", home)

	ping := otelhttp.WithRouteTag("/ping", session.ThenFunc(app.ping))
	router.Handler(http.MethodGet, "/ping", ping)

	router.Handler(http.MethodGet, "/users/register", session.ThenFunc(app.userCreate))
	router.Handler(http.MethodPost, "/users/register", session.ThenFunc(app.userCreatePost))
	router.Handler(http.MethodGet, "/users/login", session.ThenFunc(app.userLogin))
	router.Handler(http.MethodPost, "/users/login", session.ThenFunc(app.userLoginPost))
	router.Handler(http.MethodPost, "/users/logout", authenticated.ThenFunc(app.userLogoutPost))
	router.Handler(http.MethodGet, "/users/activate", session.ThenFunc(app.userActivate))
	router.Handler(http.MethodPost, "/users/activate", session.ThenFunc(app.userActivatePost))
	router.Handler(http.MethodGet, "/users/settings", authenticated.ThenFunc(app.userSettings))
	router.Handler(http.MethodPost, "/users/settings/reset", authenticated.ThenFunc(app.userPasswordResetPost))
	router.Handler(http.MethodGet, "/users/profile/:id", session.ThenFunc(app.userGet))
	router.Handler(http.MethodDelete, "/users", activated.ThenFunc(app.userDelete))

	router.Handler(http.MethodGet, "/topics", session.ThenFunc(app.topicList))
	router.Handler(http.MethodGet, "/topics/:id", session.ThenFunc(app.topicGet))
	router.Handler(http.MethodPost, "/topics", admin.ThenFunc(app.topicCreatePost))
	router.Handler(http.MethodPut, "/topics/:id", admin.ThenFunc(app.topicUpdatePost))
	router.Handler(http.MethodPost, "/topics/moderators/add/:id", admin.ThenFunc(app.topicAddModerator))
	router.Handler(http.MethodPost, "/topics/moderators/remove/:id", admin.ThenFunc(app.topicRemoveModerator))
	router.Handler(http.MethodPost, "/topics/subscribe/:id", activated.ThenFunc(app.topicSubscribe))
	router.Handler(http.MethodPost, "/topics/unsubscribe/:id", activated.ThenFunc(app.topicUnsubscribe))

	router.Handler(http.MethodGet, "/posts/:id", session.ThenFunc(app.postGet))
	router.Handler(http.MethodGet, "/posts", session.ThenFunc(app.postList))
	router.Handler(http.MethodPut, "/posts/:id", activated.ThenFunc(app.postUpdatePost))
	router.Handler(http.MethodDelete, "/posts/:id", activated.ThenFunc(app.postDelete))

	router.Handler(http.MethodGet, "/new", activated.ThenFunc(app.postCreate))
	router.Handler(http.MethodPost, "/new", activated.ThenFunc(app.postCreatePost))

	// TODO: if the user created the post/comment, display elements to let them update or delete
	router.Handler(http.MethodPost, "/posts/like/:id", activated.ThenFunc(app.postLike))
	router.Handler(http.MethodPost, "/posts/dislike/:id", activated.ThenFunc(app.postDislike))
	router.Handler(http.MethodPost, "/posts/save/:id", activated.ThenFunc(app.postSave))
	router.Handler(http.MethodPost, "/posts/unsave/:id", activated.ThenFunc(app.postUnsave))

	router.Handler(http.MethodGet, "/comments/:id", session.ThenFunc(app.commentGet))
	router.Handler(http.MethodPost, "/comments", activated.ThenFunc(app.commentCreatePost))
	router.Handler(http.MethodPut, "/comments/:id", activated.ThenFunc(app.commentUpdatePost))
	router.Handler(http.MethodDelete, "/comments/:id", activated.ThenFunc(app.commentDelete))
	router.Handler(http.MethodPost, "/comments/like/:id", activated.ThenFunc(app.commentLike))
	router.Handler(http.MethodPost, "/comments/dislike/:id", activated.ThenFunc(app.commentDislike))
	router.Handler(http.MethodPost, "/comments/save/:id", activated.ThenFunc(app.commentSave))
	router.Handler(http.MethodPost, "/comments/unsave/:id", activated.ThenFunc(app.commentUnsave))

	// TODO: unsaving a post/comment
	router.Handler(http.MethodGet, "/users/saved/posts", activated.ThenFunc(app.userPostSaved))
	router.Handler(http.MethodGet, "/users/liked/posts", activated.ThenFunc(app.userPostLiked))
	router.Handler(http.MethodGet, "/users/saved/comments", activated.ThenFunc(app.userCommentSaved))
	router.Handler(http.MethodGet, "/users/liked/comments", activated.ThenFunc(app.userCommentLiked))

	return otelhttp.NewHandler(middle.Then(router), "/")
}
