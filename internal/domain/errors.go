package domain

import "errors"

var (
	ErrUnauthorized         = errors.New("unauthorized")
	ErrMappingNotFound      = errors.New("mapping not found")
	ErrReplySessionNotFound = errors.New("reply session not found")
	ErrInvalidCommand       = errors.New("invalid command")
	ErrUserNotFound         = errors.New("user not found")
)
