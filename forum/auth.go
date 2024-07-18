package main

import (
	"net/http"
)

type contextKey string

const (
	isAuthenticatedKey = contextKey("isAuthenticated")
	authUser           = "authenticatedUserID"
)

func (app *application) isAuthenticated(r *http.Request) bool {
	isAuthenticated, ok := r.Context().Value(isAuthenticatedKey).(bool)
	if !ok {
		return false
	}
	return isAuthenticated
}
