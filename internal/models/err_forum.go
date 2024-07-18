package models

import (
	"errors"
)

var (
	ErrInvalidCredentials     = errors.New("incorrect email or password")
	ErrInconsistentData       = errors.New("data is in inconsistent state")
	ErrDuplicateEmail         = errors.New("email is already in use")
	ErrDuplicateUsername      = errors.New("username is already in use")
	ErrNoRecordFound          = errors.New("no record found")
	ErrCannotLikeAgain        = errors.New("cannot like a post or comment twice")
	ErrCannotDislikeAgain     = errors.New("cannot dislike a same post or comment twice")
	ErrConcurrencyControl     = errors.New("please wait for a few seconds and try again")
	ErrNoCommentsForPost      = errors.New("no comments found for post")
	ErrStartTransaction       = errors.New("failed to start transaction")
	ErrCommitTransaction      = errors.New("failed to commit transaction")
	ErrPrepareStatement       = errors.New("failed to prepare statement")
	ErrInsertComment          = errors.New("failed to insert comment")
	ErrInsertCommentPath      = errors.New("failed to insert comment path")
	ErrInsertCommentLike      = errors.New("failed to insert comment like")
	ErrUpdatePostCommentCount = errors.New("failed to increment number of comments on post")
)
