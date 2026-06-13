package bot

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/app"
	"github.com/KangVin/TeleRelayBot/internal/domain"
	"github.com/KangVin/TeleRelayBot/internal/ratelimit"
	"github.com/KangVin/TeleRelayBot/internal/store"

	tele "gopkg.in/telebot.v4"
)

// mockContext implements tele.Context for testing.
type mockContext struct {
	message  *tele.Message
	chat     *tele.Chat
	sender   *tele.User
	text     string
	callback *tele.Callback
	response string
}

func (m *mockContext) Bot() tele.API                        { return nil }
func (m *mockContext) Update() tele.Update                   { return tele.Update{} }
func (m *mockContext) Message() *tele.Message                { return m.message }
func (m *mockContext) Callback() *tele.Callback              { return m.callback }
func (m *mockContext) Query() *tele.Query                    { return nil }
func (m *mockContext) InlineResult() *tele.InlineResult      { return nil }
func (m *mockContext) ShippingQuery() *tele.ShippingQuery    { return nil }
func (m *mockContext) PreCheckoutQuery() *tele.PreCheckoutQuery { return nil }
func (m *mockContext) Payment() *tele.Payment                { return nil }
func (m *mockContext) Poll() *tele.Poll                      { return nil }
func (m *mockContext) PollAnswer() *tele.PollAnswer          { return nil }
func (m *mockContext) ChatMember() *tele.ChatMemberUpdate    { return nil }
func (m *mockContext) ChatJoinRequest() *tele.ChatJoinRequest { return nil }
func (m *mockContext) Migration() (int64, int64)             { return 0, 0 }
func (m *mockContext) Topic() *tele.Topic                    { return nil }
func (m *mockContext) Boost() *tele.BoostUpdated             { return nil }
func (m *mockContext) BoostRemoved() *tele.BoostRemoved      { return nil }
func (m *mockContext) PurchasedPaidMedia() *tele.PaidMediaPurchased { return nil }
func (m *mockContext) Sender() *tele.User                    { return m.sender }
func (m *mockContext) Chat() *tele.Chat                      { return m.chat }
func (m *mockContext) Recipient() tele.Recipient             { return nil }
func (m *mockContext) Text() string                          { return m.text }
func (m *mockContext) ThreadID() int                         { return 0 }
func (m *mockContext) Entities() tele.Entities               { return nil }
func (m *mockContext) Media() *tele.Media                    { return nil }
func (m *mockContext) MediaAlbum() tele.Album                 { return nil }
func (m *mockContext) MessageSig() string                    { return "" }
func (m *mockContext) Get(key string) interface{}            { return nil }
func (m *mockContext) Set(key string, val interface{})       {}
func (m *mockContext) Respond(opts ...*tele.CallbackResponse) error {
	if len(opts) > 0 {
		m.response = opts[0].Text
	}
	return nil
}
func (m *mockContext) Send(what interface{}, opts ...interface{}) error {
	if text, ok := what.(string); ok {
		m.response = text
	}
	return nil
}
func (m *mockContext) SendOrEdit(what interface{}, opts ...interface{}) error {
	return m.Send(what, opts...)
}
func (m *mockContext) Edit(what interface{}, opts ...interface{}) error {
	return m.Send(what, opts...)
}
func (m *mockContext) Delete() error { return nil }
func (m *mockContext) Reply(what interface{}, opts ...interface{}) error {
	return m.Send(what, opts...)
}
func (m *mockContext) Forward(msg tele.Editable, opts ...interface{}) error { return nil }
func (m *mockContext) ForwardTo(to tele.Recipient, opts ...interface{}) error { return nil }
func (m *mockContext) Notify(action tele.ChatAction) error { return nil }
func (m *mockContext) Banned() bool                      { return false }
func (m *mockContext) Restrict(opts ...interface{}) error { return nil }
func (m *mockContext) Promote(opts ...interface{}) error  { return nil }
func (m *mockContext) Demote(opts ...interface{}) error    { return nil }
func (m *mockContext) Kick(reason ...string) error         { return nil }
func (m *mockContext) Unban(reason ...string) error        { return nil }
func (m *mockContext) Leave() error                        { return nil }
func (m *mockContext) Stop() error                         { return nil }
func (m *mockContext) EditReplyMarkup(markup *tele.ReplyMarkup) error { return nil }
func (m *mockContext) EditCaption(caption string, opts ...interface{}) error { return nil }
func (m *mockContext) EditMedia(media tele.Media, opts ...interface{}) error { return nil }
func (m *mockContext) EditReplyMarkupAndText(text string, opts ...interface{}) error {
	return nil
}
func (m *mockContext) Args() []string                                 { return nil }
func (m *mockContext) Data() string                                   { return "" }
func (m *mockContext) SendAlbum(a tele.Album, opts ...interface{}) error { return nil }
func (m *mockContext) EditOrSend(what interface{}, opts ...interface{}) error { return m.Send(what, opts...) }
func (m *mockContext) EditOrReply(what interface{}, opts ...interface{}) error { return m.Reply(what, opts...) }
func (m *mockContext) DeleteAfter(d time.Duration) *time.Timer        { return time.NewTimer(d) }
func (m *mockContext) Ship(what ...interface{}) error                 { return nil }
func (m *mockContext) RespondText(text string) error                  { return m.Respond(&tele.CallbackResponse{Text: text}) }
func (m *mockContext) RespondAlert(text string) error                 { return m.Respond(&tele.CallbackResponse{Text: text, ShowAlert: true}) }
func (m *mockContext) EditLiveLocation(location *tele.Location, opts ...interface{}) error { return nil }
func (m *mockContext) StopLiveLocation(opts ...interface{}) error { return nil }
func (m *mockContext) Answer(resp *tele.QueryResponse) error { return nil }
func (m *mockContext) AnswerShippingQuery(ok bool, opts ...interface{}) error { return nil }
func (m *mockContext) AnswerPreCheckoutQuery(ok bool, errorMessage string) error { return nil }
func (m *mockContext) AnswerWebApp(query string, result interface{}) error { return nil }
func (m *mockContext) Accept(errorMessage ...string) error { return nil }
func (m *mockContext) Redirect(to tele.Context) error  { return nil }
func (m *mockContext) Copy(to tele.Recipient, opts ...interface{}) (*tele.Message, error) {
	return &tele.Message{ID: 999}, nil
}
func (m *mockContext) CopyEditable(what tele.Editable, opts ...interface{}) (*tele.Message, error) {
	return &tele.Message{ID: 999}, nil
}

func (m *mockContext) reset() {
	m.response = ""
}

var _ tele.Context = (*mockContext)(nil)

// mockBot implements a minimal tele.API for testing.
type mockBot struct {
	tele.API
	editedReplyMarkup *tele.ReplyMarkup
	editedMessage     *tele.Message
	lastEditText      string
	deletedMessage    interface{}
}

func (m *mockBot) EditReplyMarkup(msg tele.Editable, markup *tele.ReplyMarkup) (*tele.Message, error) {
	m.editedReplyMarkup = markup
	return &tele.Message{ID: 1}, nil
}

func (m *mockBot) Edit(msg tele.Editable, what interface{}, opts ...interface{}) (*tele.Message, error) {
	m.lastEditText = fmt.Sprintf("%v", what)
	m.editedMessage = &tele.Message{ID: 1}
	return m.editedMessage, nil
}

func (m *mockBot) Delete(msg tele.Editable) error {
	m.deletedMessage = msg
	return nil
}

func (m *mockBot) Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error) {
	return &tele.Message{ID: 100}, nil
}

func (m *mockBot) Copy(to tele.Recipient, what tele.Editable, opts ...interface{}) (*tele.Message, error) {
	return &tele.Message{ID: 101}, nil
}

// mockStore implements store.Store for testing.
type mockStore struct {
	users       map[int64]*domain.User
	mappings    map[int64]*domain.MessageMapping
	nextMapping int64
	sessions    map[int64]*domain.OwnerReplySession
	nextSession int64
	rateEvents  []mockRateEvent
	auditLogs   []domain.AuditLog
}

type mockRateEvent struct {
	TelegramID int64
	EventType  domain.RateEventType
	CreatedAt  time.Time
}

func newMockStore() *mockStore {
	return &mockStore{
		users:       make(map[int64]*domain.User),
		mappings:    make(map[int64]*domain.MessageMapping),
		nextMapping: 1,
		sessions:    make(map[int64]*domain.OwnerReplySession),
		nextSession: 1,
	}
}

func (s *mockStore) Close() error { return nil }

func (s *mockStore) Migrate(_ context.Context) error { return nil }

func (s *mockStore) UpsertUser(_ context.Context, input domain.UserUpsert) (*domain.User, error) {
	now := time.Now().UTC()
	user := &domain.User{
		ID:           input.TelegramID,
		TelegramID:   input.TelegramID,
		Username:     input.Username,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		LanguageCode: input.LanguageCode,
		Status:       domain.UserStatusNormal,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if input.LastSeenAt.IsZero() {
		user.LastSeenAt = &now
	} else {
		user.LastSeenAt = &input.LastSeenAt
	}
	if existing, ok := s.users[input.TelegramID]; ok {
		user.CreatedAt = existing.CreatedAt
		user.MessageCount = existing.MessageCount
		user.LimitedCount = existing.LimitedCount
		user.Status = existing.Status
		user.BanReason = existing.BanReason
		user.BannedUntil = existing.BannedUntil
		existing.Username = input.Username
		existing.FirstName = input.FirstName
		existing.LastName = input.LastName
		existing.LanguageCode = input.LanguageCode
		existing.UpdatedAt = now
		existing.LastSeenAt = user.LastSeenAt
		return existing, nil
	}
	s.users[input.TelegramID] = user
	return user, nil
}

func (s *mockStore) GetUser(_ context.Context, telegramID int64) (*domain.User, error) {
	user, ok := s.users[telegramID]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *mockStore) SetUserStatus(_ context.Context, telegramID int64, status domain.UserStatus, reason string) error {
	user, ok := s.users[telegramID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Status = status
	user.BanReason = reason
	user.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *mockStore) SetUserBannedUntil(_ context.Context, telegramID int64, until time.Time, reason string) error {
	user, ok := s.users[telegramID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Status = domain.UserStatusBlocked
	user.BannedUntil = &until
	user.BanReason = reason
	user.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *mockStore) ClearBan(_ context.Context, telegramID int64) error {
	user, ok := s.users[telegramID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Status = domain.UserStatusNormal
	user.BannedUntil = nil
	user.BanReason = ""
	user.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *mockStore) IncrementMessageCount(_ context.Context, telegramID int64) error {
	user, ok := s.users[telegramID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.MessageCount++
	return nil
}

func (s *mockStore) IncrementLimitedCount(_ context.Context, telegramID int64) error {
	user, ok := s.users[telegramID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.LimitedCount++
	return nil
}

func (s *mockStore) CreateMessageMapping(_ context.Context, input domain.MessageMappingCreate) (*domain.MessageMapping, error) {
	id := s.nextMapping
	s.nextMapping++
	mapping := &domain.MessageMapping{
		ID:                id,
		OwnerMessageID:    input.OwnerMessageID,
		OwnerChatID:       input.OwnerChatID,
		StrangerChatID:    input.StrangerChatID,
		StrangerMessageID: input.StrangerMessageID,
		MessageType:       input.MessageType,
		Status:            domain.MessageMappingStatusOpen,
		CreatedAt:         time.Now().UTC(),
	}
	s.mappings[id] = mapping
	return mapping, nil
}

func (s *mockStore) GetMappingByOwnerMessage(_ context.Context, ownerChatID, ownerMessageID int64) (*domain.MessageMapping, error) {
	for _, m := range s.mappings {
		if m.OwnerChatID == ownerChatID && m.OwnerMessageID == ownerMessageID {
			return m, nil
		}
	}
	return nil, domain.ErrMappingNotFound
}

func (s *mockStore) GetMappingByID(_ context.Context, id int64) (*domain.MessageMapping, error) {
	m, ok := s.mappings[id]
	if !ok {
		return nil, domain.ErrMappingNotFound
	}
	return m, nil
}

func (s *mockStore) UpdateMappingStatus(_ context.Context, id int64, status domain.MessageMappingStatus) error {
	m, ok := s.mappings[id]
	if !ok {
		return domain.ErrMappingNotFound
	}
	m.Status = status
	return nil
}

func (s *mockStore) CreateOwnerReplySession(_ context.Context, input domain.OwnerReplySessionCreate) (*domain.OwnerReplySession, error) {
	// Delete existing sessions for owner
	for id, sess := range s.sessions {
		if sess.OwnerID == input.OwnerID {
			delete(s.sessions, id)
		}
	}
	id := s.nextSession
	s.nextSession++
	session := &domain.OwnerReplySession{
		ID:               id,
		OwnerID:          input.OwnerID,
		TargetTelegramID: input.TargetTelegramID,
		MappingID:        input.MappingID,
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        input.ExpiresAt,
	}
	s.sessions[id] = session
	return session, nil
}

func (s *mockStore) GetActiveOwnerReplySession(_ context.Context, ownerID int64, now time.Time) (*domain.OwnerReplySession, error) {
	for _, sess := range s.sessions {
		if sess.OwnerID == ownerID && sess.ExpiresAt.After(now) {
			return sess, nil
		}
	}
	return nil, domain.ErrReplySessionNotFound
}

func (s *mockStore) DeleteOwnerReplySession(_ context.Context, id int64) error {
	_, ok := s.sessions[id]
	if !ok {
		return domain.ErrReplySessionNotFound
	}
	delete(s.sessions, id)
	return nil
}

func (s *mockStore) AddRateEvent(_ context.Context, telegramID int64, eventType domain.RateEventType) error {
	s.rateEvents = append(s.rateEvents, mockRateEvent{
		TelegramID: telegramID,
		EventType:  eventType,
		CreatedAt:  time.Now().UTC(),
	})
	return nil
}

func (s *mockStore) DeleteRateEventsBefore(_ context.Context, before time.Time) error {
	return nil
}

func (s *mockStore) CountRateEventsSince(_ context.Context, telegramID int64, eventType domain.RateEventType, since time.Time) (int64, error) {
	var count int64
	for _, e := range s.rateEvents {
		if e.TelegramID == telegramID && e.EventType == eventType && !e.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (s *mockStore) AddAuditLog(_ context.Context, log domain.AuditLog) error {
	s.auditLogs = append(s.auditLogs, log)
	return nil
}

func (s *mockStore) GetAuditLogs(_ context.Context, limit int) ([]domain.AuditLog, error) {
	if len(s.auditLogs) == 0 {
		return []domain.AuditLog{}, nil
	}
	n := limit
	if n > len(s.auditLogs) {
		n = len(s.auditLogs)
	}
	result := make([]domain.AuditLog, n)
	copy(result, s.auditLogs[:n])
	return result, nil
}

func (s *mockStore) Stats(_ context.Context, now time.Time) (*domain.Stats, error) {
	stats := &domain.Stats{}
	for _, u := range s.users {
		stats.TotalUsers++
		switch u.Status {
		case domain.UserStatusNormal:
			stats.NormalUsers++
		case domain.UserStatusMuted:
			stats.MutedUsers++
		case domain.UserStatusBlocked:
			stats.BlockedUsers++
		}
	}
	return stats, nil
}

func (s *mockStore) RecentUsers(_ context.Context, limit int) ([]domain.User, error) {
	var users []domain.User
	for _, u := range s.users {
		users = append(users, *u)
	}
	if len(users) > limit {
		users = users[:limit]
	}
	return users, nil
}

func (s *mockStore) BlockedUsers(_ context.Context, limit int) ([]domain.User, error) {
	var users []domain.User
	for _, u := range s.users {
		if u.Status == domain.UserStatusBlocked {
			users = append(users, *u)
		}
	}
	if len(users) > limit {
		users = users[:limit]
	}
	return users, nil
}

func mustNewHandler(t *testing.T, st store.Store, cfg app.Config) *Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelInfo}))
	rlStore, ok := st.(ratelimit.Store)
	if !ok {
		t.Fatal("store does not implement ratelimit.Store")
	}
	limiter, err := ratelimit.NewWithClock(rlStore, []ratelimit.Window{
		{Name: "test", Duration: time.Minute, Max: 100},
	}, func() time.Time { return time.Now().UTC() })
	if err != nil {
		t.Fatal(err)
	}
	api := &mockBot{}
	return &Handler{
		api:     api,
		cfg:     cfg,
		store:   st,
		log:     logger,
		sender:  NewSender(api, cfg.GlobalForwardPerSecond),
		rl:      limiter,
		rootCtx: context.Background(),
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

var _ = mustNewHandler

func TestHandleStart(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{BotToken: "token", OwnerID: 999}
	h := mustNewHandler(t, st, cfg)

	t.Run("owner gets help", func(t *testing.T) {
		c := &mockContext{
			sender: &tele.User{ID: 999},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
		}
		if err := h.handleStart(c); err != nil {
			t.Fatal(err)
		}
		if c.response == "" {
			t.Fatal("expected response")
		}
	})

	t.Run("stranger gets short message", func(t *testing.T) {
		c := &mockContext{
			sender: &tele.User{ID: 123},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
		}
		if err := h.handleStart(c); err != nil {
			t.Fatal(err)
		}
		if c.response == "" {
			t.Fatal("expected response")
		}
	})
}

func TestHandleStrangerText(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{
		BotToken:                      "token",
		OwnerID:                       999,
		MaxTextLength:                 2000,
		AutoBanOnLimitHits:            5,
		AutoBanDuration:               time.Hour,
		GlobalForwardPerSecond:        5,
		RateLimitShortWindow:          time.Minute,
		RateLimitShortMax:             100,
		RateLimitMinuteWindow:         time.Hour,
		RateLimitMinuteMax:            1000,
		RateLimitHourWindow:           24 * time.Hour,
		RateLimitHourMax:              10000,
	}
	h := mustNewHandler(t, st, cfg)

	t.Run("stranger message forwarded", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{ID: 1},
			sender:  &tele.User{ID: 123, Username: "testuser", FirstName: "Test"},
			chat:    &tele.Chat{Type: tele.ChatPrivate, ID: 123},
			text:    "Hello owner!",
		}
		if err := h.handleStrangerText(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "Message relayed. Please wait for a reply." {
			t.Fatalf("unexpected response: %q", c.response)
		}
		user, err := st.GetUser(context.Background(), 123)
		if err != nil {
			t.Fatal("user should be created")
		}
		if user.Username != "testuser" {
			t.Fatalf("username = %q, want testuser", user.Username)
		}
		if len(st.auditLogs) == 0 {
			t.Fatal("expected audit log")
		}
	})

	t.Run("blocked user rejected", func(t *testing.T) {
		st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 456})
		st.SetUserStatus(context.Background(), 456, domain.UserStatusBlocked, "spam")
		defer st.ClearBan(context.Background(), 456)

		c := &mockContext{
			message: &tele.Message{ID: 2},
			sender:  &tele.User{ID: 456},
			chat:    &tele.Chat{Type: tele.ChatPrivate, ID: 456},
			text:    "Hello",
		}
		if err := h.handleStrangerText(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "You are temporarily unable to send messages through this bot." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})

	t.Run("muted user acknowledged", func(t *testing.T) {
		st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 789})
		st.SetUserStatus(context.Background(), 789, domain.UserStatusMuted, "quiet")
		defer st.SetUserStatus(context.Background(), 789, domain.UserStatusNormal, "")

		c := &mockContext{
			message: &tele.Message{ID: 3},
			sender:  &tele.User{ID: 789},
			chat:    &tele.Chat{Type: tele.ChatPrivate, ID: 789},
			text:    "Hello",
		}
		if err := h.handleStrangerText(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "Message received." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})

	t.Run("too long text rejected", func(t *testing.T) {
		st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 111})

		h2 := mustNewHandler(t, st, app.Config{
			BotToken:                      "token",
			OwnerID:                       999,
			MaxTextLength:                 5,
			GlobalForwardPerSecond:        5,
			RateLimitShortWindow:          time.Minute,
			RateLimitShortMax:             100,
			RateLimitMinuteWindow:         time.Hour,
			RateLimitMinuteMax:            1000,
			RateLimitHourWindow:           24 * time.Hour,
			RateLimitHourMax:              10000,
		})
		c := &mockContext{
			message: &tele.Message{ID: 4},
			sender:  &tele.User{ID: 111},
			chat:    &tele.Chat{Type: tele.ChatPrivate, ID: 111},
			text:    "This is way too long text that should be rejected",
		}
		if err := h2.handleStrangerText(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "Message is too long. Please shorten it and send again." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})
}

func TestHandleOwnerReply(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{
		BotToken:               "token",
		OwnerID:                999,
		MaxTextLength:          2000,
		GlobalForwardPerSecond: 5,
	}
	h := mustNewHandler(t, st, cfg)

	// Create a user and mapping
	st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 123, Username: "user1"})
	mapping, err := st.CreateMessageMapping(context.Background(), domain.MessageMappingCreate{
		OwnerMessageID: 50,
		OwnerChatID:    999,
		StrangerChatID: 123,
		MessageType:    "text",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("reply to relayed message", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{
				ID:       100,
				Text:     "Thanks for your message",
				ReplyTo:  &tele.Message{ID: 50},
				Sender:   &tele.User{ID: 999},
				Chat:     &tele.Chat{ID: 999, Type: tele.ChatPrivate},
			},
			sender: &tele.User{ID: 999},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			text:   "Thanks for your message",
		}
		if err := h.handleOwnerReply(c); err != nil {
			t.Fatal(err)
		}
		if st.mappings[mapping.ID].Status != domain.MessageMappingStatusReplied {
			t.Fatal("mapping should be marked as replied")
		}
		if len(st.auditLogs) == 0 {
			t.Fatal("expected audit log")
		}
	})

	t.Run("reply to non-relayed message fails", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{
				ID:      101,
				Text:    "hello",
				ReplyTo: &tele.Message{ID: 99999},
			},
			sender: &tele.User{ID: 999},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			text:   "hello",
		}
		if err := h.handleOwnerReply(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "Could not find the original user. This may not be a relayed message." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})
}

func TestHandleOwnerReplySession(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{
		BotToken:               "token",
		OwnerID:                999,
		MaxTextLength:          2000,
		GlobalForwardPerSecond: 5,
	}
	h := mustNewHandler(t, st, cfg)

	st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 123, Username: "user1"})

	// Create a session
	mappingID := int64(1)
	if _, err := st.CreateOwnerReplySession(context.Background(), domain.OwnerReplySessionCreate{
		OwnerID:          999,
		TargetTelegramID: 123,
		MappingID:        &mappingID,
		ExpiresAt:        time.Now().UTC().Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("reply via session works", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{ID: 200, Text: "Reply text"},
			sender:  &tele.User{ID: 999},
			chat:    &tele.Chat{Type: tele.ChatPrivate},
			text:    "Reply text",
		}
		handled, err := h.handleOwnerReplySession(c)
		if err != nil {
			t.Fatal(err)
		}
		if !handled {
			t.Fatal("expected session to handle the message")
		}
		// Session should be deleted after successful reply
		_, err = st.GetActiveOwnerReplySession(context.Background(), 999, time.Now().UTC())
		if err == nil {
			t.Fatal("session should have been deleted after successful reply")
		}
	})

	t.Run("no session returns false", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{ID: 201, Text: "test"},
			sender:  &tele.User{ID: 999},
			chat:    &tele.Chat{Type: tele.ChatPrivate},
			text:    "test",
		}
		handled, err := h.handleOwnerReplySession(c)
		if err != nil {
			t.Fatal(err)
		}
		if handled {
			t.Fatal("expected not handled when no session")
		}
	})

	t.Run("blocked user during session", func(t *testing.T) {
		st2 := newMockStore()
		st2.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 456})
		st2.SetUserStatus(context.Background(), 456, domain.UserStatusBlocked, "spam")

		blockedSession, err := st2.CreateOwnerReplySession(context.Background(), domain.OwnerReplySessionCreate{
			OwnerID:          999,
			TargetTelegramID: 456,
			ExpiresAt:        time.Now().UTC().Add(5 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}

		h2 := mustNewHandler(t, st2, cfg)
		c := &mockContext{
			message: &tele.Message{ID: 202, Text: "hi"},
			sender:  &tele.User{ID: 999},
			chat:    &tele.Chat{Type: tele.ChatPrivate},
			text:    "hi",
		}
		handled, err := h2.handleOwnerReplySession(c)
		if err != nil {
			t.Fatal(err)
		}
		if !handled {
			t.Fatal("expected handled even when blocked")
		}
		// Session should NOT be deleted when reply is blocked
		_, err = st2.GetActiveOwnerReplySession(context.Background(), 999, time.Now().UTC())
		if err != nil {
			t.Fatal("session should still exist when blocked user prevents reply")
		}
		_ = blockedSession
	})
}

func TestHandleStrangerMedia(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{
		BotToken:                      "token",
		OwnerID:                       999,
		AllowMedia:                    true,
		AllowPhoto:                    true,
		MaxTextLength:                 2000,
		GlobalForwardPerSecond:        5,
		RateLimitShortWindow:          time.Minute,
		RateLimitShortMax:             100,
		RateLimitMinuteWindow:         time.Hour,
		RateLimitMinuteMax:            1000,
		RateLimitHourWindow:           24 * time.Hour,
		RateLimitHourMax:              10000,
	}
	h := mustNewHandler(t, st, cfg)

	t.Run("photo forwarded", func(t *testing.T) {
		c := &mockContext{
			message: &tele.Message{
				ID: 10,
				Photo: &tele.Photo{
					File: tele.File{FileID: "photo123"},
				},
				Caption: "check this out",
			},
			sender: &tele.User{ID: 123, Username: "photouser"},
			chat:   &tele.Chat{Type: tele.ChatPrivate, ID: 123},
			text:   "check this out",
		}
		if err := h.handleStrangerMedia(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "Message relayed. Please wait for a reply." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})

	t.Run("unsupported media type rejected", func(t *testing.T) {
		st2 := newMockStore()
		h2 := mustNewHandler(t, st2, app.Config{
			BotToken:               "token",
			OwnerID:                999,
			AllowMedia:             false,
			AllowPhoto:             false,
			MaxTextLength:          2000,
			GlobalForwardPerSecond: 5,
		})
		st2.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 456})
		c := &mockContext{
			message: &tele.Message{
				ID: 11,
				Photo: &tele.Photo{
					File: tele.File{FileID: "photo456"},
				},
			},
			sender: &tele.User{ID: 456},
			chat:   &tele.Chat{Type: tele.ChatPrivate, ID: 456},
		}
		if err := h2.handleStrangerMedia(c); err != nil {
			t.Fatal(err)
		}
		if c.response != "This message type is not supported yet. Please send a text message." {
			t.Fatalf("unexpected response: %q", c.response)
		}
	})
}

func TestCallbacks(t *testing.T) {
	st := newMockStore()
	cfg := app.Config{
		BotToken:               "token",
		OwnerID:                999,
		GlobalForwardPerSecond: 5,
	}
	h := mustNewHandler(t, st, cfg)

	st.UpsertUser(context.Background(), domain.UserUpsert{TelegramID: 123, Username: "user1"})
	mapping, err := st.CreateMessageMapping(context.Background(), domain.MessageMappingCreate{
		OwnerMessageID: 100,
		OwnerChatID:    999,
		StrangerChatID: 123,
		MessageType:    "text",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("quick_send callback", func(t *testing.T) {
		c := &mockContext{
			sender: &tele.User{ID: 999},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			callback: &tele.Callback{
				Unique: "quick_send",
				Data:   "quick_send:1:received",
				Message: &tele.Message{ID: 100},
			},
		}
		cb, err := ParseCallback("quick_send", "quick_send:1:received")
		if err != nil {
			t.Fatal(err)
		}
		_ = cb
		if err := h.handleCallbackQuickSend(c); err != nil {
			t.Fatal(err)
		}
		if st.mappings[mapping.ID].Status != domain.MessageMappingStatusReplied {
			t.Fatal("mapping should be replied")
		}
	})

	t.Run("ignore callback", func(t *testing.T) {
		mapping2, err := st.CreateMessageMapping(context.Background(), domain.MessageMappingCreate{
			OwnerMessageID: 200,
			OwnerChatID:    999,
			StrangerChatID: 456,
			MessageType:    "text",
		})
		if err != nil {
			t.Fatal(err)
		}
		c := &mockContext{
			sender: &tele.User{ID: 999},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			callback: &tele.Callback{
				Unique:  "ignore",
				Data:    fmt.Sprintf("%d", mapping2.ID),
				Message: &tele.Message{ID: 200},
			},
		}
		if err := h.handleCallbackIgnore(c); err != nil {
			t.Fatal(err)
		}
		if st.mappings[mapping2.ID].Status != domain.MessageMappingStatusIgnored {
			t.Fatal("mapping should be ignored")
		}
	})
}
