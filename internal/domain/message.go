package domain

import "time"

type MessageMappingStatus string

const (
	MessageMappingStatusOpen    MessageMappingStatus = "open"
	MessageMappingStatusReplied MessageMappingStatus = "replied"
	MessageMappingStatusIgnored MessageMappingStatus = "ignored"
)

type MessageMapping struct {
	ID                int64
	OwnerMessageID    int64
	OwnerChatID       int64
	StrangerChatID    int64
	StrangerMessageID *int64
	MessageType       string
	Status            MessageMappingStatus
	CreatedAt         time.Time
}

type MessageMappingCreate struct {
	OwnerMessageID    int64
	OwnerChatID       int64
	StrangerChatID    int64
	StrangerMessageID *int64
	MessageType       string
}

type OwnerReplySession struct {
	ID               int64
	OwnerID          int64
	TargetTelegramID int64
	MappingID        *int64
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

type OwnerReplySessionCreate struct {
	OwnerID          int64
	TargetTelegramID int64
	MappingID        *int64
	ExpiresAt        time.Time
}

type RateEventType string

const (
	RateEventTypeMessage RateEventType = "message"
	RateEventTypeLimited RateEventType = "limited"
	RateEventTypeAutoBan RateEventType = "auto_ban"
)

type AuditAction string

const (
	AuditActionBan         AuditAction = "ban"
	AuditActionUnban       AuditAction = "unban"
	AuditActionMute        AuditAction = "mute"
	AuditActionUnmute      AuditAction = "unmute"
	AuditActionReply       AuditAction = "reply"
	AuditActionForward     AuditAction = "forward"
	AuditActionIgnore      AuditAction = "ignore"
	AuditActionRateLimited AuditAction = "rate_limited"
	AuditActionAutoBan     AuditAction = "auto_ban"
	AuditActionQuickReply  AuditAction = "quick_reply"
)

type AuditLog struct {
	ID        int64
	ActorID   int64
	Action    AuditAction
	TargetID  *int64
	Detail    string
	CreatedAt time.Time
}

type Stats struct {
	TotalUsers    int64
	NormalUsers   int64
	MutedUsers    int64
	BlockedUsers  int64
	TodayMessages int64
	TodayLimited  int64
	TodayReplies  int64
}
