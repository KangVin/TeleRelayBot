package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/app"
	"github.com/KangVin/TeleRelayBot/internal/domain"
	"github.com/KangVin/TeleRelayBot/internal/ratelimit"
	"github.com/KangVin/TeleRelayBot/internal/store"
	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	bot    *tele.Bot
	cfg    app.Config
	store  *store.SQLiteStore
	log    *slog.Logger
	sender *Sender
	rl     *ratelimit.Limiter
}

func New(b *tele.Bot, cfg app.Config, st *store.SQLiteStore, logger *slog.Logger) (*Handler, error) {
	limiter, err := ratelimit.New(cfg, st)
	if err != nil {
		return nil, err
	}
	return &Handler{
		bot:    b,
		cfg:    cfg,
		store:  st,
		log:    logger,
		sender: NewSender(b, cfg.GlobalForwardPerSecond),
		rl:     limiter,
	}, nil
}

func (h *Handler) Register() {
	h.bot.Handle("/start", h.handleStart)
	h.bot.Handle("/help", h.handleHelp)
	h.bot.Handle("/stats", h.ownerOnly(h.handleStats))
	h.bot.Handle("/recent", h.ownerOnly(h.handleRecent))
	h.bot.Handle("/user", h.ownerOnly(h.handleUserCommand))
	h.bot.Handle("/ban", h.ownerOnly(h.handleBanCommand))
	h.bot.Handle("/unban", h.ownerOnly(h.handleUnbanCommand))
	h.bot.Handle("/mute", h.ownerOnly(h.handleMuteCommand))
	h.bot.Handle("/unmute", h.ownerOnly(h.handleUnmuteCommand))
	h.bot.Handle("/blocklist", h.ownerOnly(h.handleBlocklist))
	h.bot.Handle("/reply", h.ownerOnly(h.handleReplyCommand))
	h.bot.Handle("/cancelreply", h.ownerOnly(h.handleCancelReplyCommand))
	h.bot.Handle(tele.OnText, h.handleText)
	h.bot.Handle(tele.OnPhoto, h.handleMedia)
	h.bot.Handle(tele.OnDocument, h.handleMedia)
	h.bot.Handle(tele.OnVoice, h.handleMedia)
	h.bot.Handle(tele.OnVideo, h.handleMedia)
	h.bot.Handle(tele.OnAudio, h.handleMedia)
	h.bot.Handle(tele.OnSticker, h.handleMedia)

	h.registerCallbacks()
	h.registerCommands()
}

func (h *Handler) registerCommands() {
	commands := []tele.Command{
		{Text: "start", Description: "Start the bot and show welcome message"},
		{Text: "help", Description: "Show available commands"},
		{Text: "stats", Description: "Show aggregate user and message statistics"},
		{Text: "recent", Description: "List recent users"},
		{Text: "user", Description: "Show stored user details by ID"},
		{Text: "ban", Description: "Block a user"},
		{Text: "unban", Description: "Unblock a user"},
		{Text: "mute", Description: "Mute a user without blocking"},
		{Text: "unmute", Description: "Unmute a muted user"},
		{Text: "blocklist", Description: "List blocked users"},
		{Text: "reply", Description: "Send a message to a user by ID"},
		{Text: "cancelreply", Description: "Cancel active reply mode"},
	}
	if err := h.bot.SetCommands(commands); err != nil {
		h.log.Error("register bot commands", slog.Any("error", err))
	}
}

func (h *Handler) handleStart(c tele.Context) error {
	if h.isOwner(c) {
		return c.Send(ownerHelpText())
	}
	return c.Send("Hello. Send a private message here and I will relay it to the owner. Please avoid sending too frequently.")
}

func (h *Handler) handleHelp(c tele.Context) error {
	if h.isOwner(c) {
		return c.Send(ownerHelpText())
	}
	return c.Send("Send a text message in this private chat. The owner can reply through the bot.")
}

func (h *Handler) handleText(c tele.Context) error {
	if c.Message() == nil || c.Sender() == nil || c.Chat() == nil {
		return nil
	}
	if strings.HasPrefix(c.Text(), "/") {
		return nil
	}
	if c.Chat().Type != tele.ChatPrivate {
		return nil
	}
	if h.isOwner(c) {
		if c.Message().ReplyTo != nil {
			return h.handleOwnerReply(c)
		}
		if handled, err := h.handleOwnerReplySession(c); handled || err != nil {
			return err
		}
		return c.Send("Reply to a relayed user message, or use /reply <telegram_id> <message>.")
	}
	return h.handleStrangerText(c)
}

func (h *Handler) handleStrangerText(c tele.Context) error {
	ctx := context.Background()
	msg := c.Message()
	user := userFromTele(c.Sender())
	if _, err := h.store.UpsertUser(ctx, user); err != nil {
		h.log.Error("upsert user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}

	dbUser, err := h.store.GetUser(ctx, c.Sender().ID)
	if err != nil {
		h.log.Error("get user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if err := h.refreshExpiredTemporaryBan(ctx, dbUser); err != nil {
		h.log.Error("refresh temporary ban", slog.Any("error", err), slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message could not be processed.")
	}
	if blockedNow(dbUser, time.Now().UTC()) {
		return c.Send("You are temporarily unable to send messages through this bot.")
	}
	if dbUser.Status == domain.UserStatusMuted {
		h.log.Info("muted user message", slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message received.")
	}
	if len(c.Text()) > h.cfg.MaxTextLength {
		return c.Send("Message is too long. Please shorten it and send again.")
	}
	allowed, err := h.rl.Allow(ctx, c.Sender().ID)
	if err != nil {
		h.log.Error("rate limit check", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if !allowed {
		if err := h.autoBanAfterLimit(ctx, c.Sender().ID); err != nil {
			h.log.Error("auto ban after limit", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
		}
		h.log.Info("rate limited", slog.Int64("telegram_id", c.Sender().ID))
		return c.Send("You are sending too frequently. Please try again later.")
	}

	ownerText := FormatOwnerMessage(OwnerMessage{
		TelegramID:   dbUser.TelegramID,
		Username:     dbUser.Username,
		FirstName:    dbUser.FirstName,
		LastName:     dbUser.LastName,
		LanguageCode: dbUser.LanguageCode,
		Text:         c.Text(),
		SentAt:       time.Now().UTC(),
	})
	sent, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, ownerText)
	if err != nil {
		h.log.Error("forward to owner", slog.Any("error", err))
		return c.Send("Message volume is high. Please try again later.")
	}

	strangerMessageID := int64(msg.ID)
	mapping, err := h.store.CreateMessageMapping(ctx, domain.MessageMappingCreate{
		OwnerMessageID:    int64(sent.ID),
		OwnerChatID:       h.cfg.OwnerID,
		StrangerChatID:    c.Sender().ID,
		StrangerMessageID: &strangerMessageID,
		MessageType:       "text",
	})
	if err != nil {
		h.log.Error("create mapping", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	h.attachOwnerKeyboard(ctx, sent, mapping.ID, c.Sender().ID, string(dbUser.Status))
	_ = h.store.IncrementMessageCount(ctx, c.Sender().ID)
	ownerID := h.cfg.OwnerID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: c.Sender().ID, Action: domain.AuditActionForward, TargetID: &ownerID})
	h.log.Info("forwarded to owner", slog.Int64("telegram_id", c.Sender().ID), slog.Int("owner_message_id", sent.ID))
	return c.Send("Message relayed. Please wait for a reply.")
}

func (h *Handler) attachOwnerKeyboard(ctx context.Context, ownerMessage *tele.Message, mappingID, userID int64, status string) {
	markup := BuildOwnerMessageKeyboard(mappingID, userID, status)
	if _, err := h.bot.EditReplyMarkup(ownerMessage, markup); err == nil {
		return
	} else {
		h.log.Error("attach owner keyboard", slog.Any("error", err), slog.Int64("mapping_id", mappingID), slog.Int64("user_id", userID))
	}

	panelText := fmt.Sprintf("Actions for relayed message %d from user %d.", ownerMessage.ID, userID)
	if _, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, panelText, markup); err != nil {
		h.log.Error("send owner action panel", slog.Any("error", err), slog.Int64("mapping_id", mappingID), slog.Int64("user_id", userID))
	}
}

func (h *Handler) handleOwnerReply(c tele.Context) error {
	ctx := context.Background()
	mapping, err := h.store.GetMappingByOwnerMessage(ctx, h.cfg.OwnerID, int64(c.Message().ReplyTo.ID))
	if err != nil {
		if errors.Is(err, domain.ErrMappingNotFound) {
			return c.Send("Could not find the original user. This may not be a relayed message.")
		}
		h.log.Error("get mapping", slog.Any("error", err))
		return c.Send("Reply could not be processed.")
	}
	user, err := h.store.GetUser(ctx, mapping.StrangerChatID)
	if err != nil {
		h.log.Error("get reply target", slog.Any("error", err))
		return c.Send("Reply target could not be loaded.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if len(c.Text()) > h.cfg.MaxTextLength {
		return c.Send("Reply is too long.")
	}
	if _, err := h.sender.SendUser(mapping.StrangerChatID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, c.Text())); err != nil {
		h.log.Error("send owner reply", slog.Any("error", err))
		return c.Send("Reply could not be sent.")
	}
	_ = h.store.UpdateMappingStatus(ctx, mapping.ID, domain.MessageMappingStatusReplied)
	targetID := mapping.StrangerChatID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID})
	h.log.Info("owner replied", slog.Int64("target_id", mapping.StrangerChatID))
	return c.Send(fmt.Sprintf("Replied to user %d.", mapping.StrangerChatID))
}

func (h *Handler) handleOwnerReplySession(c tele.Context) (bool, error) {
	ctx := context.Background()
	session, err := h.store.GetActiveOwnerReplySession(ctx, h.cfg.OwnerID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrReplySessionNotFound) {
			return false, nil
		}
		h.log.Error("get owner reply session", slog.Any("error", err))
		return true, c.Send("Reply session could not be loaded.")
	}
	defer func() {
		if err := h.store.DeleteOwnerReplySession(ctx, session.ID); err != nil && !errors.Is(err, domain.ErrReplySessionNotFound) {
			h.log.Error("delete owner reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
		}
	}()

	if len(c.Text()) > h.cfg.MaxTextLength {
		return true, c.Send("Reply is too long.")
	}
	user, err := h.store.GetUser(ctx, session.TargetTelegramID)
	if err != nil {
		return true, c.Send("User not found.")
	}
	if user.Status == domain.UserStatusBlocked {
		return true, c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.SendUser(session.TargetTelegramID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, c.Text())); err != nil {
		h.log.Error("send session reply", slog.Any("error", err), slog.Int64("target_id", session.TargetTelegramID))
		return true, c.Send("Reply could not be sent.")
	}
	if session.MappingID != nil {
		_ = h.store.UpdateMappingStatus(ctx, *session.MappingID, domain.MessageMappingStatusReplied)
	}
	targetID := session.TargetTelegramID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID})
	h.log.Info("owner replied via session", slog.Int64("target_id", session.TargetTelegramID))
	return true, c.Send(fmt.Sprintf("Replied to user %d.", session.TargetTelegramID))
}

func (h *Handler) handleMedia(c tele.Context) error {
	if c.Message() == nil || c.Sender() == nil || c.Chat() == nil {
		return nil
	}
	if c.Chat().Type != tele.ChatPrivate {
		return nil
	}
	if h.isOwner(c) {
		if c.Message().ReplyTo != nil {
			return h.handleOwnerMediaReply(c)
		}
		if handled, err := h.handleOwnerMediaReplySession(c); handled || err != nil {
			return err
		}
		return c.Send("Reply to a relayed user message with media, or use /reply <telegram_id> <message> for text.")
	}
	return h.handleStrangerMedia(c)
}

func (h *Handler) handleStrangerMedia(c tele.Context) error {
	ctx := context.Background()
	msg := c.Message()
	media := detectMedia(msg, h.cfg)
	if media.Type == "" || !media.Allowed {
		return c.Send("This message type is not supported yet. Please send a text message.")
	}
	if len(media.Caption) > h.cfg.MaxTextLength {
		return c.Send("Message is too long. Please shorten it and send again.")
	}

	if _, err := h.store.UpsertUser(ctx, userFromTele(c.Sender())); err != nil {
		h.log.Error("upsert media user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	dbUser, err := h.store.GetUser(ctx, c.Sender().ID)
	if err != nil {
		h.log.Error("get media user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if err := h.refreshExpiredTemporaryBan(ctx, dbUser); err != nil {
		h.log.Error("refresh temporary ban", slog.Any("error", err), slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message could not be processed.")
	}
	if blockedNow(dbUser, time.Now().UTC()) {
		return c.Send("You are temporarily unable to send messages through this bot.")
	}
	if dbUser.Status == domain.UserStatusMuted {
		h.log.Info("muted user media", slog.Int64("telegram_id", dbUser.TelegramID), slog.String("media_type", media.Type))
		return c.Send("Message received.")
	}

	allowed, err := h.rl.Allow(ctx, c.Sender().ID)
	if err != nil {
		h.log.Error("media rate limit check", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if !allowed {
		if err := h.autoBanAfterLimit(ctx, c.Sender().ID); err != nil {
			h.log.Error("auto ban after media limit", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
		}
		h.log.Info("media rate limited", slog.Int64("telegram_id", c.Sender().ID))
		return c.Send("You are sending too frequently. Please try again later.")
	}

	ownerText := FormatOwnerMediaMessage(dbUser, msg.ID, media.Type, media.Caption)
	meta, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, ownerText)
	if err != nil {
		h.log.Error("send media metadata", slog.Any("error", err))
		return c.Send("Message volume is high. Please try again later.")
	}
	strangerMessageID := int64(msg.ID)
	mapping, err := h.store.CreateMessageMapping(ctx, domain.MessageMappingCreate{
		OwnerMessageID:    int64(meta.ID),
		OwnerChatID:       h.cfg.OwnerID,
		StrangerChatID:    c.Sender().ID,
		StrangerMessageID: &strangerMessageID,
		MessageType:       media.Type,
	})
	if err != nil {
		h.log.Error("create media mapping", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	h.attachOwnerKeyboard(ctx, meta, mapping.ID, c.Sender().ID, string(dbUser.Status))
	if _, err := h.sender.CopyOwner(ctx, h.cfg.OwnerID, msg); err != nil {
		h.log.Error("copy media to owner", slog.Any("error", err), slog.String("media_type", media.Type))
		return c.Send("Message could not be copied. Please try again later.")
	}
	_ = h.store.IncrementMessageCount(ctx, c.Sender().ID)
	ownerID := h.cfg.OwnerID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: c.Sender().ID, Action: domain.AuditActionForward, TargetID: &ownerID, Detail: media.Type})
	h.log.Info("forwarded media to owner", slog.Int64("telegram_id", c.Sender().ID), slog.String("media_type", media.Type), slog.Int("owner_message_id", meta.ID))
	return c.Send("Message relayed. Please wait for a reply.")
}

func (h *Handler) handleOwnerMediaReply(c tele.Context) error {
	ctx := context.Background()
	media := detectMedia(c.Message(), h.cfg)
	if media.Type == "" || !media.Allowed {
		return c.Send("This media type is not enabled.")
	}
	if len(media.Caption) > h.cfg.MaxTextLength {
		return c.Send("Reply is too long.")
	}
	mapping, err := h.store.GetMappingByOwnerMessage(ctx, h.cfg.OwnerID, int64(c.Message().ReplyTo.ID))
	if err != nil {
		if errors.Is(err, domain.ErrMappingNotFound) {
			return c.Send("Could not find the original user. This may not be a relayed message.")
		}
		h.log.Error("get media reply mapping", slog.Any("error", err))
		return c.Send("Reply could not be processed.")
	}
	user, err := h.store.GetUser(ctx, mapping.StrangerChatID)
	if err != nil {
		h.log.Error("get media reply target", slog.Any("error", err))
		return c.Send("Reply target could not be loaded.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.CopyUser(mapping.StrangerChatID, c.Message()); err != nil {
		h.log.Error("copy owner media reply", slog.Any("error", err), slog.String("media_type", media.Type))
		return c.Send("Reply could not be sent.")
	}
	_ = h.store.UpdateMappingStatus(ctx, mapping.ID, domain.MessageMappingStatusReplied)
	targetID := mapping.StrangerChatID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID, Detail: media.Type})
	h.log.Info("owner replied with media", slog.Int64("target_id", mapping.StrangerChatID), slog.String("media_type", media.Type))
	return c.Send(fmt.Sprintf("Replied to user %d.", mapping.StrangerChatID))
}

func (h *Handler) handleOwnerMediaReplySession(c tele.Context) (bool, error) {
	ctx := context.Background()
	media := detectMedia(c.Message(), h.cfg)
	if media.Type == "" || !media.Allowed {
		return true, c.Send("This media type is not enabled.")
	}
	if len(media.Caption) > h.cfg.MaxTextLength {
		return true, c.Send("Reply is too long.")
	}

	session, err := h.store.GetActiveOwnerReplySession(ctx, h.cfg.OwnerID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrReplySessionNotFound) {
			return false, nil
		}
		h.log.Error("get owner media reply session", slog.Any("error", err))
		return true, c.Send("Reply session could not be loaded.")
	}
	defer func() {
		if err := h.store.DeleteOwnerReplySession(ctx, session.ID); err != nil && !errors.Is(err, domain.ErrReplySessionNotFound) {
			h.log.Error("delete owner media reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
		}
	}()

	user, err := h.store.GetUser(ctx, session.TargetTelegramID)
	if err != nil {
		return true, c.Send("User not found.")
	}
	if user.Status == domain.UserStatusBlocked {
		return true, c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.CopyUser(session.TargetTelegramID, c.Message()); err != nil {
		h.log.Error("copy session media reply", slog.Any("error", err), slog.String("media_type", media.Type))
		return true, c.Send("Reply could not be sent.")
	}
	if session.MappingID != nil {
		_ = h.store.UpdateMappingStatus(ctx, *session.MappingID, domain.MessageMappingStatusReplied)
	}
	targetID := session.TargetTelegramID
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID, Detail: media.Type})
	h.log.Info("owner replied via media session", slog.Int64("target_id", session.TargetTelegramID), slog.String("media_type", media.Type))
	return true, c.Send(fmt.Sprintf("Replied to user %d.", session.TargetTelegramID))
}

func (h *Handler) handleReplyCommand(c tele.Context) error {
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		return c.Send("Usage: /reply <telegram_id> <message>")
	}
	targetID := cmd.UserID
	text := cmd.Message
	if len(text) > h.cfg.MaxTextLength {
		return c.Send("Reply is too long.")
	}
	user, err := h.store.GetUser(context.Background(), targetID)
	if err != nil {
		return c.Send("User not found.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.SendUser(targetID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, text)); err != nil {
		h.log.Error("send reply command", slog.Any("error", err))
		return c.Send("Reply could not be sent.")
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID})
	return c.Send(fmt.Sprintf("Replied to user %d.", targetID))
}

func (h *Handler) handleCancelReplyCommand(c tele.Context) error {
	if _, err := ParseCommand(c.Text()); err != nil {
		return c.Send("Usage: /cancelreply")
	}
	session, err := h.store.GetActiveOwnerReplySession(context.Background(), h.cfg.OwnerID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrReplySessionNotFound) {
			return c.Send("No active reply mode.")
		}
		h.log.Error("get active reply session", slog.Any("error", err))
		return c.Send("Reply mode could not be cancelled.")
	}
	if err := h.store.DeleteOwnerReplySession(context.Background(), session.ID); err != nil {
		h.log.Error("delete active reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
		return c.Send("Reply mode could not be cancelled.")
	}
	return c.Send("Reply mode cancelled.")
}

func (h *Handler) handleBanCommand(c tele.Context) error {
	id, reason, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /ban <telegram_id> [reason]")
	}
	if err := h.store.SetUserStatus(context.Background(), id, domain.UserStatusBlocked, trimMax(reason, 500)); err != nil {
		return c.Send("User not found.")
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionBan, TargetID: &id, Detail: reason})
	return c.Send(fmt.Sprintf("User %d banned.", id))
}

func (h *Handler) handleUnbanCommand(c tele.Context) error {
	id, _, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /unban <telegram_id>")
	}
	if err := h.store.ClearBan(context.Background(), id); err != nil {
		return c.Send("User not found.")
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionUnban, TargetID: &id})
	return c.Send(fmt.Sprintf("User %d unbanned.", id))
}

func (h *Handler) handleMuteCommand(c tele.Context) error {
	id, reason, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /mute <telegram_id> [reason]")
	}
	if err := h.store.SetUserStatus(context.Background(), id, domain.UserStatusMuted, trimMax(reason, 500)); err != nil {
		return c.Send("User not found.")
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionMute, TargetID: &id, Detail: reason})
	return c.Send(fmt.Sprintf("User %d muted.", id))
}

func (h *Handler) handleUnmuteCommand(c tele.Context) error {
	id, _, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /unmute <telegram_id>")
	}
	if err := h.store.SetUserStatus(context.Background(), id, domain.UserStatusNormal, ""); err != nil {
		return c.Send("User not found.")
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionUnmute, TargetID: &id})
	return c.Send(fmt.Sprintf("User %d unmuted.", id))
}

func (h *Handler) handleUserCommand(c tele.Context) error {
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		return c.Send("Usage: /user <telegram_id>")
	}
	user, err := h.store.GetUser(context.Background(), cmd.UserID)
	if err != nil {
		return c.Send("User not found.")
	}
	return c.Send(formatDomainUserInfo(user))
}

func (h *Handler) handleStats(c tele.Context) error {
	stats, err := h.store.Stats(context.Background(), time.Now().UTC())
	if err != nil {
		h.log.Error("stats", slog.Any("error", err))
		return c.Send("Stats unavailable.")
	}
	return c.Send(formatDomainStats(stats))
}

func (h *Handler) handleRecent(c tele.Context) error {
	limit := 10
	cmd, _ := ParseCommand(c.Text())
	if cmd.Limit > 0 {
		limit = min(cmd.Limit, 50)
	}
	users, err := h.store.RecentUsers(context.Background(), limit)
	if err != nil {
		return c.Send("Recent users unavailable.")
	}
	return c.Send(FormatUserList("Recent users", users))
}

func (h *Handler) handleBlocklist(c tele.Context) error {
	users, err := h.store.BlockedUsers(context.Background(), 50)
	if err != nil {
		return c.Send("Blocklist unavailable.")
	}
	return c.Send(FormatUserList("Blocked users", users))
}

func ownerHelpText() string {
	return strings.Join([]string{
		"Owner panel",
		"",
		"/stats",
		"/recent [limit]",
		"/user <telegram_id>",
		"/ban <telegram_id> [reason]",
		"/unban <telegram_id>",
		"/mute <telegram_id> [reason]",
		"/unmute <telegram_id>",
		"/blocklist",
		"/reply <telegram_id> <message>",
		"/cancelreply",
		"",
		"Reply directly to a relayed message to answer that user.",
	}, "\n")
}

func userFromTele(u *tele.User) domain.UserUpsert {
	return domain.UserUpsert{
		TelegramID:   u.ID,
		Username:     u.Username,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		LanguageCode: u.LanguageCode,
		LastSeenAt:   time.Now().UTC(),
	}
}

func blockedNow(user *domain.User, now time.Time) bool {
	return user.Status == domain.UserStatusBlocked || (user.BannedUntil != nil && user.BannedUntil.After(now))
}

func (h *Handler) refreshExpiredTemporaryBan(ctx context.Context, user *domain.User) error {
	if user.BannedUntil == nil || user.BannedUntil.After(time.Now().UTC()) {
		return nil
	}
	if err := h.store.ClearBan(ctx, user.TelegramID); err != nil {
		return err
	}
	user.Status = domain.UserStatusNormal
	user.BannedUntil = nil
	user.BanReason = ""
	return nil
}

func (h *Handler) autoBanAfterLimit(ctx context.Context, telegramID int64) error {
	user, err := h.store.GetUser(ctx, telegramID)
	if err != nil {
		return err
	}
	if user.LimitedCount < int64(h.cfg.AutoBanOnLimitHits) {
		return nil
	}
	if user.BannedUntil != nil && user.BannedUntil.After(time.Now().UTC()) {
		return nil
	}

	until := time.Now().UTC().Add(h.cfg.AutoBanDuration)
	reason := "automatic temporary ban after repeated rate limits"
	if err := h.store.SetUserBannedUntil(ctx, telegramID, until, reason); err != nil {
		return err
	}
	_ = h.store.AddRateEvent(ctx, telegramID, domain.RateEventTypeAutoBan)
	_ = h.store.AddAuditLog(ctx, domain.AuditLog{
		ActorID:  h.cfg.OwnerID,
		Action:   domain.AuditActionAutoBan,
		TargetID: &telegramID,
		Detail:   reason,
	})
	h.log.Info("auto banned user", slog.Int64("telegram_id", telegramID), slog.Time("banned_until", until))
	return nil
}

func parseIDReason(text string) (int64, string, bool) {
	cmd, err := ParseCommand(text)
	if err != nil {
		return 0, "", false
	}
	return cmd.UserID, cmd.Reason, true
}

func trimMax(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func applyOwnerPrefix(prefix, text string) string {
	if prefix == "" {
		return text
	}
	return prefix + text
}
