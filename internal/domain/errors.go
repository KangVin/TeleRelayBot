package domain

import "errors"

var (
	ErrUnauthorized           = errors.New("unauthorized")
	ErrUserBlocked            = errors.New("user blocked")
	ErrUserMuted              = errors.New("user muted")
	ErrRateLimited            = errors.New("rate limited")
	ErrMessageTooLong         = errors.New("message too long")
	ErrUnsupportedMessageType = errors.New("unsupported message type")
	ErrMappingNotFound        = errors.New("mapping not found")
	ErrReplySessionNotFound   = errors.New("reply session not found")
	ErrInvalidCommand         = errors.New("invalid command")
	ErrUserNotFound           = errors.New("user not found")
)
