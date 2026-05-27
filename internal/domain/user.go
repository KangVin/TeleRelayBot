package domain

import "time"

type UserStatus string

const (
	UserStatusNormal  UserStatus = "normal"
	UserStatusMuted   UserStatus = "muted"
	UserStatusBlocked UserStatus = "blocked"
)

type User struct {
	ID           int64
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastSeenAt   *time.Time
	MessageCount int64
	LimitedCount int64
	BanReason    string
	BannedUntil  *time.Time
}

type UserUpsert struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	LastSeenAt   time.Time
}
