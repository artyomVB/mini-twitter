package storage

import "errors"

var ErrPostNotFound = errors.New("post not found")
var ErrParseToken = errors.New("error parse token")
var ErrForbiddenAccess = errors.New("forbidden access")
var ErrCacheMiss = errors.New("cache miss")
var ErrInvalidSubscribe = errors.New("cannot subscribe on this user")
