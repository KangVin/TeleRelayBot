package store

import (
	"context"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

type Store interface {
	Open(ctx context.Context) error
	Close() error
	Migrate(ctx context.Context) error

	UpsertUser(ctx context.Context, user domain.UserUpsert) (*domain.User, error)
	GetUser(ctx context.Context, telegramID int64) (*domain.User, error)
	SetUserStatus(ctx context.Context, telegramID int64, status domain.UserStatus, reason string) error
	SetUserBannedUntil(ctx context.Context, telegramID int64, until time.Time, reason string) error
	ClearBan(ctx context.Context, telegramID int64) error
	IncrementMessageCount(ctx context.Context, telegramID int64) error
	IncrementLimitedCount(ctx context.Context, telegramID int64) error

	CreateMessageMapping(ctx context.Context, mapping domain.MessageMappingCreate) (*domain.MessageMapping, error)
	GetMappingByOwnerMessage(ctx context.Context, ownerChatID, ownerMessageID int64) (*domain.MessageMapping, error)
	GetMappingByID(ctx context.Context, id int64) (*domain.MessageMapping, error)
	UpdateMappingStatus(ctx context.Context, id int64, status domain.MessageMappingStatus) error
	CreateOwnerReplySession(ctx context.Context, session domain.OwnerReplySessionCreate) (*domain.OwnerReplySession, error)
	GetActiveOwnerReplySession(ctx context.Context, ownerID int64, now time.Time) (*domain.OwnerReplySession, error)
	DeleteOwnerReplySession(ctx context.Context, id int64) error

	AddRateEvent(ctx context.Context, telegramID int64, eventType domain.RateEventType) error
	CountRateEventsSince(ctx context.Context, telegramID int64, eventType domain.RateEventType, since time.Time) (int64, error)
	AddAuditLog(ctx context.Context, log domain.AuditLog) error
	Stats(ctx context.Context, now time.Time) (*domain.Stats, error)
	RecentUsers(ctx context.Context, limit int) ([]domain.User, error)
	BlockedUsers(ctx context.Context, limit int) ([]domain.User, error)
}
