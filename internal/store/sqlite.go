package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	path string
	db   *sql.DB
}

func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	s := &SQLiteStore{path: path}
	if err := s.Open(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func NewSQLiteStore(path string) *SQLiteStore {
	return &SQLiteStore{path: path}
}

func (s *SQLiteStore) Open(ctx context.Context) error {
	if s.db != nil {
		return nil
	}
	if s.path == "" {
		return errors.New("sqlite path is required")
	}
	if err := ensureDir(s.path); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;"); err != nil {
		_ = db.Close()
		return fmt.Errorf("configure sqlite: %w", err)
	}
	s.db = db
	return nil
}

func (s *SQLiteStore) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpsertUser(ctx context.Context, input domain.UserUpsert) (*domain.User, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	now := utcNow()
	lastSeen := input.LastSeenAt.UTC()
	if input.LastSeenAt.IsZero() {
		lastSeen = now
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO users (
	telegram_id, username, first_name, last_name, language_code,
	status, created_at, updated_at, last_seen_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(telegram_id) DO UPDATE SET
	username = excluded.username,
	first_name = excluded.first_name,
	last_name = excluded.last_name,
	language_code = excluded.language_code,
	updated_at = excluded.updated_at,
	last_seen_at = excluded.last_seen_at
`, input.TelegramID, input.Username, input.FirstName, input.LastName, input.LanguageCode,
		domain.UserStatusNormal, now, now, lastSeen)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return s.GetUser(ctx, input.TelegramID)
}

func (s *SQLiteStore) GetUser(ctx context.Context, telegramID int64) (*domain.User, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	user, err := scanUser(s.db.QueryRowContext(ctx, `
SELECT id, telegram_id, username, first_name, last_name, language_code, status,
	created_at, updated_at, last_seen_at, message_count, limited_count, ban_reason, banned_until
FROM users
WHERE telegram_id = ?
`, telegramID))
	if err != nil {
		return nil, translateUserErr(err)
	}
	return user, nil
}

func (s *SQLiteStore) SetUserStatus(ctx context.Context, telegramID int64, status domain.UserStatus, reason string) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE users
SET status = ?, ban_reason = ?, updated_at = ?
WHERE telegram_id = ?
`, status, reason, utcNow(), telegramID)
	return requireAffected(res, err, domain.ErrUserNotFound, "set user status")
}

func (s *SQLiteStore) SetUserBannedUntil(ctx context.Context, telegramID int64, until time.Time, reason string) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE users
SET status = ?, banned_until = ?, ban_reason = ?, updated_at = ?
WHERE telegram_id = ?
`, domain.UserStatusBlocked, until.UTC(), reason, utcNow(), telegramID)
	return requireAffected(res, err, domain.ErrUserNotFound, "set user banned until")
}

func (s *SQLiteStore) ClearBan(ctx context.Context, telegramID int64) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE users
SET status = ?, banned_until = NULL, ban_reason = '', updated_at = ?
WHERE telegram_id = ?
`, domain.UserStatusNormal, utcNow(), telegramID)
	return requireAffected(res, err, domain.ErrUserNotFound, "clear ban")
}

func (s *SQLiteStore) IncrementMessageCount(ctx context.Context, telegramID int64) error {
	return s.incrementUserCounter(ctx, telegramID, "message_count")
}

func (s *SQLiteStore) IncrementLimitedCount(ctx context.Context, telegramID int64) error {
	return s.incrementUserCounter(ctx, telegramID, "limited_count")
}

func (s *SQLiteStore) CreateMessageMapping(ctx context.Context, input domain.MessageMappingCreate) (*domain.MessageMapping, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO message_mappings (
	owner_message_id, owner_chat_id, stranger_chat_id, stranger_message_id,
	message_type, status, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
`, input.OwnerMessageID, input.OwnerChatID, input.StrangerChatID, optionalInt(input.StrangerMessageID),
		input.MessageType, domain.MessageMappingStatusOpen, utcNow())
	if err != nil {
		return nil, fmt.Errorf("create message mapping: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create message mapping id: %w", err)
	}
	return s.GetMappingByID(ctx, id)
}

func (s *SQLiteStore) GetMappingByOwnerMessage(ctx context.Context, ownerChatID, ownerMessageID int64) (*domain.MessageMapping, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	mapping, err := scanMapping(s.db.QueryRowContext(ctx, `
SELECT id, owner_message_id, owner_chat_id, stranger_chat_id, stranger_message_id,
	message_type, status, created_at
FROM message_mappings
WHERE owner_chat_id = ? AND owner_message_id = ?
`, ownerChatID, ownerMessageID))
	if err != nil {
		return nil, translateMappingErr(err)
	}
	return mapping, nil
}

func (s *SQLiteStore) GetMappingByID(ctx context.Context, id int64) (*domain.MessageMapping, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	mapping, err := scanMapping(s.db.QueryRowContext(ctx, `
SELECT id, owner_message_id, owner_chat_id, stranger_chat_id, stranger_message_id,
	message_type, status, created_at
FROM message_mappings
WHERE id = ?
`, id))
	if err != nil {
		return nil, translateMappingErr(err)
	}
	return mapping, nil
}

func (s *SQLiteStore) UpdateMappingStatus(ctx context.Context, id int64, status domain.MessageMappingStatus) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE message_mappings SET status = ? WHERE id = ?", status, id)
	return requireAffected(res, err, domain.ErrMappingNotFound, "update mapping status")
}

func (s *SQLiteStore) CreateOwnerReplySession(ctx context.Context, input domain.OwnerReplySessionCreate) (*domain.OwnerReplySession, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	if input.ExpiresAt.IsZero() {
		return nil, errors.New("reply session expires_at is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin reply session: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, "DELETE FROM owner_reply_sessions WHERE owner_id = ?", input.OwnerID); err != nil {
		return nil, fmt.Errorf("clear existing reply session: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO owner_reply_sessions (owner_id, target_telegram_id, mapping_id, created_at, expires_at)
VALUES (?, ?, ?, ?, ?)
`, input.OwnerID, input.TargetTelegramID, optionalInt(input.MappingID), utcNow(), input.ExpiresAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("create owner reply session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create owner reply session id: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit owner reply session: %w", err)
	}
	return s.getOwnerReplySessionByID(ctx, id)
}

func (s *SQLiteStore) GetActiveOwnerReplySession(ctx context.Context, ownerID int64, now time.Time) (*domain.OwnerReplySession, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM owner_reply_sessions WHERE owner_id = ? AND expires_at <= ?", ownerID, now.UTC())
	session, err := scanOwnerReplySession(s.db.QueryRowContext(ctx, `
SELECT id, owner_id, target_telegram_id, mapping_id, created_at, expires_at
FROM owner_reply_sessions
WHERE owner_id = ? AND expires_at > ?
ORDER BY created_at DESC
LIMIT 1
`, ownerID, now.UTC()))
	if err != nil {
		return nil, translateReplySessionErr(err)
	}
	return session, nil
}

func (s *SQLiteStore) DeleteOwnerReplySession(ctx context.Context, id int64) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM owner_reply_sessions WHERE id = ?", id)
	return requireAffected(res, err, domain.ErrReplySessionNotFound, "delete owner reply session")
}

func (s *SQLiteStore) AddRateEvent(ctx context.Context, telegramID int64, eventType domain.RateEventType) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO rate_events (telegram_id, event_type, created_at)
VALUES (?, ?, ?)
`, telegramID, eventType, utcNow())
	if err != nil {
		return fmt.Errorf("add rate event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CountRateEventsSince(ctx context.Context, telegramID int64, eventType domain.RateEventType, since time.Time) (int64, error) {
	if err := s.requireOpen(); err != nil {
		return 0, err
	}
	var count int64
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM rate_events
WHERE telegram_id = ? AND event_type = ? AND created_at >= ?
`, telegramID, eventType, since.UTC()).Scan(&count); err != nil {
		return 0, fmt.Errorf("count rate events: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) AddAuditLog(ctx context.Context, log domain.AuditLog) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	createdAt := log.CreatedAt.UTC()
	if log.CreatedAt.IsZero() {
		createdAt = utcNow()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO audit_logs (actor_id, action, target_id, detail, created_at)
VALUES (?, ?, ?, ?, ?)
`, log.ActorID, log.Action, optionalInt(log.TargetID), log.Detail, createdAt)
	if err != nil {
		return fmt.Errorf("add audit log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Stats(ctx context.Context, now time.Time) (*domain.Stats, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	start := startOfUTCDay(now)
	stats := &domain.Stats{}
	if err := s.db.QueryRowContext(ctx, `
SELECT
	COUNT(*),
	COALESCE(SUM(CASE WHEN status = 'normal' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status = 'muted' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END), 0)
FROM users
`).Scan(&stats.TotalUsers, &stats.NormalUsers, &stats.MutedUsers, &stats.BlockedUsers); err != nil {
		return nil, fmt.Errorf("stats users: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT
	COALESCE(SUM(CASE WHEN event_type = 'message' THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN event_type = 'limited' THEN 1 ELSE 0 END), 0)
FROM rate_events
WHERE created_at >= ?
`, start).Scan(&stats.TodayMessages, &stats.TodayLimited); err != nil {
		return nil, fmt.Errorf("stats rate events: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM audit_logs
WHERE action IN ('reply', 'quick_reply') AND created_at >= ?
`, start).Scan(&stats.TodayReplies); err != nil {
		return nil, fmt.Errorf("stats replies: %w", err)
	}
	return stats, nil
}

func (s *SQLiteStore) RecentUsers(ctx context.Context, limit int) ([]domain.User, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, telegram_id, username, first_name, last_name, language_code, status,
	created_at, updated_at, last_seen_at, message_count, limited_count, ban_reason, banned_until
FROM users
ORDER BY COALESCE(last_seen_at, updated_at, created_at) DESC
LIMIT ?
`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("recent users: %w", err)
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *SQLiteStore) BlockedUsers(ctx context.Context, limit int) ([]domain.User, error) {
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, telegram_id, username, first_name, last_name, language_code, status,
	created_at, updated_at, last_seen_at, message_count, limited_count, ban_reason, banned_until
FROM users
WHERE status = ?
ORDER BY updated_at DESC
LIMIT ?
`, domain.UserStatusBlocked, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("blocked users: %w", err)
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *SQLiteStore) incrementUserCounter(ctx context.Context, telegramID int64, column string) error {
	if err := s.requireOpen(); err != nil {
		return err
	}
	switch column {
	case "message_count", "limited_count":
	default:
		return fmt.Errorf("unsupported counter column %q", column)
	}
	res, err := s.db.ExecContext(ctx, fmt.Sprintf(`
UPDATE users
SET %s = %s + 1, updated_at = ?
WHERE telegram_id = ?
`, column, column), utcNow(), telegramID)
	return requireAffected(res, err, domain.ErrUserNotFound, "increment user counter")
}

func (s *SQLiteStore) requireOpen() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	return nil
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (*domain.User, error) {
	var user domain.User
	var username, firstName, lastName, languageCode, banReason sql.NullString
	var lastSeenAt, bannedUntil sql.NullTime
	if err := row.Scan(
		&user.ID,
		&user.TelegramID,
		&username,
		&firstName,
		&lastName,
		&languageCode,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastSeenAt,
		&user.MessageCount,
		&user.LimitedCount,
		&banReason,
		&bannedUntil,
	); err != nil {
		return nil, err
	}
	user.Username = username.String
	user.FirstName = firstName.String
	user.LastName = lastName.String
	user.LanguageCode = languageCode.String
	user.BanReason = banReason.String
	if lastSeenAt.Valid {
		t := lastSeenAt.Time.UTC()
		user.LastSeenAt = &t
	}
	if bannedUntil.Valid {
		t := bannedUntil.Time.UTC()
		user.BannedUntil = &t
	}
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return &user, nil
}

func scanUsers(rows *sql.Rows) ([]domain.User, error) {
	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func scanMapping(row interface {
	Scan(dest ...any) error
}) (*domain.MessageMapping, error) {
	var mapping domain.MessageMapping
	var strangerMessageID sql.NullInt64
	if err := row.Scan(
		&mapping.ID,
		&mapping.OwnerMessageID,
		&mapping.OwnerChatID,
		&mapping.StrangerChatID,
		&strangerMessageID,
		&mapping.MessageType,
		&mapping.Status,
		&mapping.CreatedAt,
	); err != nil {
		return nil, err
	}
	if strangerMessageID.Valid {
		id := strangerMessageID.Int64
		mapping.StrangerMessageID = &id
	}
	mapping.CreatedAt = mapping.CreatedAt.UTC()
	return &mapping, nil
}

func scanOwnerReplySession(row interface {
	Scan(dest ...any) error
}) (*domain.OwnerReplySession, error) {
	var session domain.OwnerReplySession
	var mappingID sql.NullInt64
	if err := row.Scan(
		&session.ID,
		&session.OwnerID,
		&session.TargetTelegramID,
		&mappingID,
		&session.CreatedAt,
		&session.ExpiresAt,
	); err != nil {
		return nil, err
	}
	if mappingID.Valid {
		id := mappingID.Int64
		session.MappingID = &id
	}
	session.CreatedAt = session.CreatedAt.UTC()
	session.ExpiresAt = session.ExpiresAt.UTC()
	return &session, nil
}

func (s *SQLiteStore) getOwnerReplySessionByID(ctx context.Context, id int64) (*domain.OwnerReplySession, error) {
	session, err := scanOwnerReplySession(s.db.QueryRowContext(ctx, `
SELECT id, owner_id, target_telegram_id, mapping_id, created_at, expires_at
FROM owner_reply_sessions
WHERE id = ?
`, id))
	if err != nil {
		return nil, translateReplySessionErr(err)
	}
	return session, nil
}

func requireAffected(res sql.Result, err error, notFound error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", op, err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}

func translateUserErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	return fmt.Errorf("get user: %w", err)
}

func translateMappingErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrMappingNotFound
	}
	return fmt.Errorf("get mapping: %w", err)
}

func translateReplySessionErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrReplySessionNotFound
	}
	return fmt.Errorf("get reply session: %w", err)
}

func ensureDir(path string) error {
	if strings.Contains(path, ":memory:") || strings.HasPrefix(path, "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	return nil
}

func optionalInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func utcNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 50 {
		return 50
	}
	return limit
}
