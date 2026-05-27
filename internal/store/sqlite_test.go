package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

func TestSQLiteStoreUserLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t, ctx)

	user, err := st.UpsertUser(ctx, domain.UserUpsert{
		TelegramID:   123,
		Username:     "alice",
		FirstName:    "Alice",
		LanguageCode: "en",
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if user.Status != domain.UserStatusNormal {
		t.Fatalf("status = %q, want %q", user.Status, domain.UserStatusNormal)
	}

	if err := st.IncrementMessageCount(ctx, 123); err != nil {
		t.Fatalf("increment message count: %v", err)
	}
	if err := st.SetUserStatus(ctx, 123, domain.UserStatusMuted, "quiet"); err != nil {
		t.Fatalf("mute user: %v", err)
	}
	user, err = st.GetUser(ctx, 123)
	if err != nil {
		t.Fatalf("get muted user: %v", err)
	}
	if user.Status != domain.UserStatusMuted || user.MessageCount != 1 {
		t.Fatalf("user after mute/count = %+v", user)
	}

	until := time.Now().UTC().Add(time.Hour)
	if err := st.SetUserBannedUntil(ctx, 123, until, "spam"); err != nil {
		t.Fatalf("ban user: %v", err)
	}
	user, err = st.GetUser(ctx, 123)
	if err != nil {
		t.Fatalf("get banned user: %v", err)
	}
	if user.Status != domain.UserStatusBlocked || user.BannedUntil == nil || user.BanReason != "spam" {
		t.Fatalf("user after ban = %+v", user)
	}

	if err := st.ClearBan(ctx, 123); err != nil {
		t.Fatalf("clear ban: %v", err)
	}
	user, err = st.GetUser(ctx, 123)
	if err != nil {
		t.Fatalf("get unbanned user: %v", err)
	}
	if user.Status != domain.UserStatusNormal || user.BannedUntil != nil || user.BanReason != "" {
		t.Fatalf("user after clear ban = %+v", user)
	}
}

func TestSQLiteStoreMessageMappingLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t, ctx)

	strangerMessageID := int64(44)
	mapping, err := st.CreateMessageMapping(ctx, domain.MessageMappingCreate{
		OwnerMessageID:    11,
		OwnerChatID:       22,
		StrangerChatID:    33,
		StrangerMessageID: &strangerMessageID,
		MessageType:       "text",
	})
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}
	if mapping.Status != domain.MessageMappingStatusOpen {
		t.Fatalf("mapping status = %q, want open", mapping.Status)
	}

	byOwnerMessage, err := st.GetMappingByOwnerMessage(ctx, 22, 11)
	if err != nil {
		t.Fatalf("get mapping by owner message: %v", err)
	}
	if byOwnerMessage.ID != mapping.ID || byOwnerMessage.StrangerChatID != 33 {
		t.Fatalf("mapping by owner message = %+v, want id %d stranger 33", byOwnerMessage, mapping.ID)
	}

	if err := st.UpdateMappingStatus(ctx, mapping.ID, domain.MessageMappingStatusReplied); err != nil {
		t.Fatalf("update mapping status: %v", err)
	}
	byID, err := st.GetMappingByID(ctx, mapping.ID)
	if err != nil {
		t.Fatalf("get mapping by id: %v", err)
	}
	if byID.Status != domain.MessageMappingStatusReplied {
		t.Fatalf("mapping status = %q, want replied", byID.Status)
	}

	_, err = st.GetMappingByOwnerMessage(ctx, 22, 99)
	if !errors.Is(err, domain.ErrMappingNotFound) {
		t.Fatalf("missing mapping err = %v, want ErrMappingNotFound", err)
	}
}

func TestSQLiteStoreOwnerReplySessionLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t, ctx)

	mappingID := int64(42)
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	session, err := st.CreateOwnerReplySession(ctx, domain.OwnerReplySessionCreate{
		OwnerID:          99,
		TargetTelegramID: 123,
		MappingID:        &mappingID,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		t.Fatalf("create reply session: %v", err)
	}
	if session.OwnerID != 99 || session.TargetTelegramID != 123 || session.MappingID == nil || *session.MappingID != mappingID {
		t.Fatalf("reply session = %+v", session)
	}

	active, err := st.GetActiveOwnerReplySession(ctx, 99, time.Now().UTC())
	if err != nil {
		t.Fatalf("get active reply session: %v", err)
	}
	if active.ID != session.ID {
		t.Fatalf("active session id = %d, want %d", active.ID, session.ID)
	}

	replacement, err := st.CreateOwnerReplySession(ctx, domain.OwnerReplySessionCreate{
		OwnerID:          99,
		TargetTelegramID: 456,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		t.Fatalf("replace reply session: %v", err)
	}
	active, err = st.GetActiveOwnerReplySession(ctx, 99, time.Now().UTC())
	if err != nil {
		t.Fatalf("get replacement reply session: %v", err)
	}
	if active.ID != replacement.ID || active.TargetTelegramID != 456 {
		t.Fatalf("replacement active session = %+v", active)
	}

	if _, err := st.GetActiveOwnerReplySession(ctx, 99, expiresAt.Add(time.Second)); !errors.Is(err, domain.ErrReplySessionNotFound) {
		t.Fatalf("expired session err = %v, want ErrReplySessionNotFound", err)
	}
	if err := st.DeleteOwnerReplySession(ctx, replacement.ID); !errors.Is(err, domain.ErrReplySessionNotFound) {
		t.Fatalf("delete expired session err = %v, want ErrReplySessionNotFound", err)
	}
}

func TestSQLiteStoreRateAuditStatsAndLists(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t, ctx)

	if _, err := st.UpsertUser(ctx, domain.UserUpsert{TelegramID: 1, Username: "one"}); err != nil {
		t.Fatalf("upsert user 1: %v", err)
	}
	if _, err := st.UpsertUser(ctx, domain.UserUpsert{TelegramID: 2, Username: "two"}); err != nil {
		t.Fatalf("upsert user 2: %v", err)
	}
	if err := st.SetUserStatus(ctx, 2, domain.UserStatusBlocked, "spam"); err != nil {
		t.Fatalf("block user 2: %v", err)
	}
	if err := st.AddRateEvent(ctx, 1, domain.RateEventTypeMessage); err != nil {
		t.Fatalf("add message rate event: %v", err)
	}
	if err := st.AddRateEvent(ctx, 1, domain.RateEventTypeLimited); err != nil {
		t.Fatalf("add limited rate event: %v", err)
	}
	targetID := int64(1)
	if err := st.AddAuditLog(ctx, domain.AuditLog{ActorID: 99, Action: domain.AuditActionReply, TargetID: &targetID}); err != nil {
		t.Fatalf("add audit log: %v", err)
	}

	count, err := st.CountRateEventsSince(ctx, 1, domain.RateEventTypeMessage, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("count rate events: %v", err)
	}
	if count != 1 {
		t.Fatalf("message event count = %d, want 1", count)
	}

	stats, err := st.Stats(ctx, time.Now())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalUsers != 2 || stats.BlockedUsers != 1 || stats.TodayMessages != 1 || stats.TodayLimited != 1 || stats.TodayReplies != 1 {
		t.Fatalf("stats = %+v", stats)
	}

	recent, err := st.RecentUsers(ctx, 50)
	if err != nil {
		t.Fatalf("recent users: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("recent users len = %d, want 2", len(recent))
	}
	blocked, err := st.BlockedUsers(ctx, 50)
	if err != nil {
		t.Fatalf("blocked users: %v", err)
	}
	if len(blocked) != 1 || blocked[0].TelegramID != 2 {
		t.Fatalf("blocked users = %+v", blocked)
	}
}

func openTestStore(t *testing.T, ctx context.Context) *SQLiteStore {
	t.Helper()

	st, err := Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("second migrate store: %v", err)
	}
	return st
}
