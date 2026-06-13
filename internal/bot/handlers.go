package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type contextKey string

const contextKeyTraceID contextKey = "trace_id"

type Handler struct {
	bot   *tele.Bot
	api   tele.API
	cfg   app.Config
	store store.Store
	log   *slog.Logger
	sender *Sender
	rl     *ratelimit.Limiter
	rootCtx context.Context
}

func New(b *tele.Bot, cfg app.Config, st store.Store, logger *slog.Logger, rootCtx context.Context) (*Handler, error) {
	rlStore, ok := st.(ratelimit.Store)
	if !ok {
		return nil, errors.New("store does not implement ratelimit.Store")
	}
	limiter, err := ratelimit.New(cfg, rlStore)
	if err != nil {
		return nil, err
	}
	return &Handler{
		bot:     b,
		api:     b,
		cfg:     cfg,
		store:   st,
		log:     logger,
		sender:  NewSender(b, cfg.GlobalForwardPerSecond),
		rl:      limiter,
		rootCtx: rootCtx,
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
	h.bot.Handle("/audit", h.ownerOnly(h.handleAudit))
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
		{Text: "audit", Description: "View recent audit log entries"},
		{Text: "reply", Description: "Send a message to a user by ID"},
		{Text: "cancelreply", Description: "Cancel active reply mode"},
	}
	if err := h.bot.SetCommands(commands); err != nil {
		h.log.Error("register bot commands", slog.Any("error", err))
	}
}

func (h *Handler) handleStart(c tele.Context) error {
	if h.isOwner(c) {
		return c.Send(ownerHelpText(), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}
	return c.Send("Hello. Send a private message here and I will relay it to the owner. Please avoid sending too frequently.")
}

func (h *Handler) handleHelp(c tele.Context) error {
	if h.isOwner(c) {
		return c.Send(ownerHelpText(), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
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
	ctx, cancel := h.requestContext()
	defer cancel()
	msg := c.Message()
	user := userFromTele(c.Sender())
	if _, err := h.store.UpsertUser(ctx, user); err != nil {
		h.logWith(ctx).Error("upsert user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}

	dbUser, err := h.store.GetUser(ctx, c.Sender().ID)
	if err != nil {
		h.logWith(ctx).Error("get user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if err := h.refreshExpiredTemporaryBan(ctx, dbUser, time.Now().UTC()); err != nil {
		h.logWith(ctx).Error("refresh temporary ban", slog.Any("error", err), slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message could not be processed.")
	}
	if blockedNow(dbUser, time.Now().UTC()) {
		return c.Send("You are temporarily unable to send messages through this bot.")
	}
	if dbUser.Status == domain.UserStatusMuted {
		h.logWith(ctx).Info("muted user message", slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message received.")
	}
	if len(c.Text()) > h.cfg.MaxTextLength {
		return c.Send("Message is too long. Please shorten it and send again.")
	}
	allowed, err := h.rl.Allow(ctx, c.Sender().ID)
	if err != nil {
		h.logWith(ctx).Error("rate limit check", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if !allowed {
		if err := h.autoBanAfterLimit(ctx, c.Sender().ID); err != nil {
			h.logWith(ctx).Error("auto ban after limit", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
		}
		h.logWith(ctx).Info("rate limited", slog.Int64("telegram_id", c.Sender().ID))
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
	sent, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, ownerText, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	if err != nil {
		h.logWith(ctx).Error("forward to owner", slog.Any("error", err))
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
		h.logWith(ctx).Error("create mapping", slog.Any("error", err))
		if delErr := h.api.Delete(sent); delErr != nil {
			h.logWith(ctx).Error("delete orphaned owner message", slog.Any("error", delErr))
		}
		return c.Send("Message could not be processed.")
	}
	h.attachOwnerKeyboard(ctx, sent, mapping.ID, c.Sender().ID, string(dbUser.Status))
	if err := h.store.IncrementMessageCount(ctx, c.Sender().ID); err != nil {
		h.logWith(ctx).Error("increment message count", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
	}
	ownerID := h.cfg.OwnerID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: c.Sender().ID, Action: domain.AuditActionForward, TargetID: &ownerID}); err != nil {
		h.logWith(ctx).Error("add forward audit log", slog.Any("error", err), slog.Int64("actor_id", c.Sender().ID))
	}
	h.logWith(ctx).Info("forwarded to owner", slog.Int64("telegram_id", c.Sender().ID), slog.Int("owner_message_id", sent.ID))
	return c.Send("Message relayed. Please wait for a reply.")
}

func (h *Handler) attachOwnerKeyboard(ctx context.Context, ownerMessage *tele.Message, mappingID, userID int64, status string) {
	markup := BuildOwnerMessageKeyboard(mappingID, userID, status)
	if _, err := h.api.EditReplyMarkup(ownerMessage, markup); err == nil {
		return
	} else {
		h.logWith(ctx).Error("attach owner keyboard", slog.Any("error", err), slog.Int64("mapping_id", mappingID), slog.Int64("user_id", userID))
	}

	panelText := fmt.Sprintf("Actions for relayed message %d from user %d.", ownerMessage.ID, userID)
	if _, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, panelText, markup); err != nil {
		h.logWith(ctx).Error("send owner action panel", slog.Any("error", err), slog.Int64("mapping_id", mappingID), slog.Int64("user_id", userID))
	}
}

func (h *Handler) handleOwnerReply(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	mapping, err := h.store.GetMappingByOwnerMessage(ctx, h.cfg.OwnerID, int64(c.Message().ReplyTo.ID))
	if err != nil {
		if errors.Is(err, domain.ErrMappingNotFound) {
			return c.Send("Could not find the original user. This may not be a relayed message.")
		}
		h.logWith(ctx).Error("get mapping", slog.Any("error", err))
		return c.Send("Reply could not be processed.")
	}
	user, err := h.store.GetUser(ctx, mapping.StrangerChatID)
	if err != nil {
		h.logWith(ctx).Error("get reply target", slog.Any("error", err))
		return c.Send("Reply target could not be loaded.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if len(c.Text()) > h.cfg.MaxTextLength {
		return c.Send("Reply is too long.")
	}
	if _, err := h.sender.SendUser(ctx, mapping.StrangerChatID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, c.Text())); err != nil {
		h.logWith(ctx).Error("send owner reply", slog.Any("error", err))
		return c.Send("Reply could not be sent.")
	}
	if err := h.store.UpdateMappingStatus(ctx, mapping.ID, domain.MessageMappingStatusReplied); err != nil {
		h.logWith(ctx).Error("update mapping status", slog.Any("error", err), slog.Int64("mapping_id", mapping.ID))
	}
	targetID := mapping.StrangerChatID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID}); err != nil {
		h.logWith(ctx).Error("add reply audit log", slog.Any("error", err), slog.Int64("target_id", targetID))
	}
	h.logWith(ctx).Info("owner replied", slog.Int64("target_id", mapping.StrangerChatID))
	return c.Send(fmt.Sprintf("Replied to user %d.", mapping.StrangerChatID))
}

func (h *Handler) handleOwnerReplySession(c tele.Context) (bool, error) {
	ctx, cancel := h.requestContext()
	defer cancel()
	session, err := h.store.GetActiveOwnerReplySession(ctx, h.cfg.OwnerID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrReplySessionNotFound) {
			return false, nil
		}
		h.logWith(ctx).Error("get owner reply session", slog.Any("error", err))
		return true, c.Send("Reply session could not be loaded.")
	}

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
	if _, err := h.sender.SendUser(ctx, session.TargetTelegramID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, c.Text())); err != nil {
		h.logWith(ctx).Error("send session reply", slog.Any("error", err), slog.Int64("target_id", session.TargetTelegramID))
		return true, c.Send("Reply could not be sent.")
	}
	if session.MappingID != nil {
		if err := h.store.UpdateMappingStatus(ctx, *session.MappingID, domain.MessageMappingStatusReplied); err != nil {
			h.logWith(ctx).Error("update session mapping status", slog.Any("error", err), slog.Int64("mapping_id", *session.MappingID))
		}
	}
	targetID := session.TargetTelegramID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID}); err != nil {
		h.logWith(ctx).Error("add session reply audit log", slog.Any("error", err), slog.Int64("target_id", targetID))
	}
	if err := h.store.DeleteOwnerReplySession(ctx, session.ID); err != nil && !errors.Is(err, domain.ErrReplySessionNotFound) {
		h.logWith(ctx).Error("delete owner reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
	}
	h.logWith(ctx).Info("owner replied via session", slog.Int64("target_id", session.TargetTelegramID))
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
	ctx, cancel := h.requestContext()
	defer cancel()
	msg := c.Message()
	media := detectMedia(msg, h.cfg)
	if media.Type == "" || !media.Allowed {
		return c.Send("This message type is not supported yet. Please send a text message.")
	}
	if len(media.Caption) > h.cfg.MaxTextLength {
		return c.Send("Message is too long. Please shorten it and send again.")
	}

	if _, err := h.store.UpsertUser(ctx, userFromTele(c.Sender())); err != nil {
		h.logWith(ctx).Error("upsert media user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	dbUser, err := h.store.GetUser(ctx, c.Sender().ID)
	if err != nil {
		h.logWith(ctx).Error("get media user", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if err := h.refreshExpiredTemporaryBan(ctx, dbUser, time.Now().UTC()); err != nil {
		h.logWith(ctx).Error("refresh temporary ban", slog.Any("error", err), slog.Int64("telegram_id", dbUser.TelegramID))
		return c.Send("Message could not be processed.")
	}
	if blockedNow(dbUser, time.Now().UTC()) {
		return c.Send("You are temporarily unable to send messages through this bot.")
	}
	if dbUser.Status == domain.UserStatusMuted {
		h.logWith(ctx).Info("muted user media", slog.Int64("telegram_id", dbUser.TelegramID), slog.String("media_type", media.Type))
		return c.Send("Message received.")
	}

	allowed, err := h.rl.Allow(ctx, c.Sender().ID)
	if err != nil {
		h.logWith(ctx).Error("media rate limit check", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if !allowed {
		if err := h.autoBanAfterLimit(ctx, c.Sender().ID); err != nil {
			h.logWith(ctx).Error("auto ban after media limit", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
		}
		h.logWith(ctx).Info("media rate limited", slog.Int64("telegram_id", c.Sender().ID))
		return c.Send("You are sending too frequently. Please try again later.")
	}

	ownerText := FormatOwnerMediaMessage(dbUser, msg.ID, media.Type, media.Caption)
	meta, err := h.sender.SendOwner(ctx, h.cfg.OwnerID, ownerText, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	if err != nil {
		h.logWith(ctx).Error("send media metadata", slog.Any("error", err))
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
		h.logWith(ctx).Error("create media mapping", slog.Any("error", err))
		return c.Send("Message could not be processed.")
	}
	if _, err := h.sender.CopyOwner(ctx, h.cfg.OwnerID, msg); err != nil {
		h.logWith(ctx).Error("copy media to owner", slog.Any("error", err), slog.String("media_type", media.Type))
		if delErr := h.api.Delete(meta); delErr != nil {
			h.logWith(ctx).Error("delete orphaned media metadata", slog.Any("error", delErr))
		}
		return c.Send("Message could not be copied. Please try again later.")
	}
	h.attachOwnerKeyboard(ctx, meta, mapping.ID, c.Sender().ID, string(dbUser.Status))
	if err := h.store.IncrementMessageCount(ctx, c.Sender().ID); err != nil {
		h.logWith(ctx).Error("increment media message count", slog.Any("error", err), slog.Int64("telegram_id", c.Sender().ID))
	}
	ownerID := h.cfg.OwnerID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: c.Sender().ID, Action: domain.AuditActionForward, TargetID: &ownerID, Detail: media.Type}); err != nil {
		h.logWith(ctx).Error("add media forward audit log", slog.Any("error", err), slog.Int64("actor_id", c.Sender().ID))
	}
	h.logWith(ctx).Info("forwarded media to owner", slog.Int64("telegram_id", c.Sender().ID), slog.String("media_type", media.Type), slog.Int("owner_message_id", meta.ID))
	return c.Send("Message relayed. Please wait for a reply.")
}

func (h *Handler) handleOwnerMediaReply(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
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
		h.logWith(ctx).Error("get media reply mapping", slog.Any("error", err))
		return c.Send("Reply could not be processed.")
	}
	user, err := h.store.GetUser(ctx, mapping.StrangerChatID)
	if err != nil {
		h.logWith(ctx).Error("get media reply target", slog.Any("error", err))
		return c.Send("Reply target could not be loaded.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.CopyUser(ctx, mapping.StrangerChatID, c.Message()); err != nil {
		h.logWith(ctx).Error("copy owner media reply", slog.Any("error", err), slog.String("media_type", media.Type))
		return c.Send("Reply could not be sent.")
	}
	if err := h.store.UpdateMappingStatus(ctx, mapping.ID, domain.MessageMappingStatusReplied); err != nil {
		h.logWith(ctx).Error("update media reply mapping status", slog.Any("error", err), slog.Int64("mapping_id", mapping.ID))
	}
	targetID := mapping.StrangerChatID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID, Detail: media.Type}); err != nil {
		h.logWith(ctx).Error("add media reply audit log", slog.Any("error", err), slog.Int64("target_id", targetID))
	}
	h.logWith(ctx).Info("owner replied with media", slog.Int64("target_id", mapping.StrangerChatID), slog.String("media_type", media.Type))
	return c.Send(fmt.Sprintf("Replied to user %d.", mapping.StrangerChatID))
}

func (h *Handler) handleOwnerMediaReplySession(c tele.Context) (bool, error) {
	ctx, cancel := h.requestContext()
	defer cancel()
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
		h.logWith(ctx).Error("get owner media reply session", slog.Any("error", err))
		return true, c.Send("Reply session could not be loaded.")
	}

	user, err := h.store.GetUser(ctx, session.TargetTelegramID)
	if err != nil {
		return true, c.Send("User not found.")
	}
	if user.Status == domain.UserStatusBlocked {
		return true, c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.CopyUser(ctx, session.TargetTelegramID, c.Message()); err != nil {
		h.logWith(ctx).Error("copy session media reply", slog.Any("error", err), slog.String("media_type", media.Type))
		return true, c.Send("Reply could not be sent.")
	}
	if session.MappingID != nil {
		if err := h.store.UpdateMappingStatus(ctx, *session.MappingID, domain.MessageMappingStatusReplied); err != nil {
			h.logWith(ctx).Error("update session media mapping status", slog.Any("error", err), slog.Int64("mapping_id", *session.MappingID))
		}
	}
	targetID := session.TargetTelegramID
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID, Detail: media.Type}); err != nil {
		h.logWith(ctx).Error("add session media reply audit log", slog.Any("error", err), slog.Int64("target_id", targetID))
	}
	if err := h.store.DeleteOwnerReplySession(ctx, session.ID); err != nil && !errors.Is(err, domain.ErrReplySessionNotFound) {
		h.logWith(ctx).Error("delete owner media reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
	}
	h.logWith(ctx).Info("owner replied via media session", slog.Int64("target_id", session.TargetTelegramID), slog.String("media_type", media.Type))
	return true, c.Send(fmt.Sprintf("Replied to user %d.", session.TargetTelegramID))
}

func (h *Handler) handleReplyCommand(c tele.Context) error {
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		return c.Send("Usage: /reply <telegram_id> <message>")
	}
	ctx, cancel := h.requestContext()
	defer cancel()
	targetID := cmd.UserID
	text := cmd.Message
	if len(text) > h.cfg.MaxTextLength {
		return c.Send("Reply is too long.")
	}
	user, err := h.store.GetUser(ctx, targetID)
	if err != nil {
		return c.Send("User not found.")
	}
	if user.Status == domain.UserStatusBlocked {
		return c.Send("User is blocked; reply was not sent.")
	}
	if _, err := h.sender.SendUser(ctx, targetID, applyOwnerPrefix(h.cfg.OwnerReplyPrefix, text)); err != nil {
		h.logWith(ctx).Error("send reply command", slog.Any("error", err))
		return c.Send("Reply could not be sent.")
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionReply, TargetID: &targetID}); err != nil {
		h.logWith(ctx).Error("add reply command audit log", slog.Any("error", err), slog.Int64("target_id", targetID))
	}
	return c.Send(fmt.Sprintf("Replied to user %d.", targetID))
}

func (h *Handler) handleCancelReplyCommand(c tele.Context) error {
	if _, err := ParseCommand(c.Text()); err != nil {
		return c.Send("Usage: /cancelreply")
	}
	ctx, cancel := h.requestContext()
	defer cancel()
	session, err := h.store.GetActiveOwnerReplySession(ctx, h.cfg.OwnerID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrReplySessionNotFound) {
			return c.Send("No active reply mode.")
		}
		h.logWith(ctx).Error("get active reply session", slog.Any("error", err))
		return c.Send("Reply mode could not be cancelled.")
	}
	if err := h.store.DeleteOwnerReplySession(ctx, session.ID); err != nil {
		h.logWith(ctx).Error("delete active reply session", slog.Any("error", err), slog.Int64("session_id", session.ID))
		return c.Send("Reply mode could not be cancelled.")
	}
	return c.Send("Reply mode cancelled.")
}

func (h *Handler) handleBanCommand(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	id, reason, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /ban <telegram_id> [reason]")
	}
	if err := h.store.SetUserStatus(ctx, id, domain.UserStatusBlocked, trimMax(reason, 500)); err != nil {
		return c.Send("User not found.")
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionBan, TargetID: &id, Detail: reason}); err != nil {
		h.logWith(ctx).Error("add ban audit log", slog.Any("error", err), slog.Int64("target_id", id))
	}
	return c.Send(fmt.Sprintf("User %d banned.", id))
}

func (h *Handler) handleUnbanCommand(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	id, _, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /unban <telegram_id>")
	}
	if err := h.store.ClearBan(ctx, id); err != nil {
		return c.Send("User not found.")
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionUnban, TargetID: &id}); err != nil {
		h.logWith(ctx).Error("add unban audit log", slog.Any("error", err), slog.Int64("target_id", id))
	}
	return c.Send(fmt.Sprintf("User %d unbanned.", id))
}

func (h *Handler) handleMuteCommand(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	id, reason, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /mute <telegram_id> [reason]")
	}
	if err := h.store.SetUserStatus(ctx, id, domain.UserStatusMuted, trimMax(reason, 500)); err != nil {
		return c.Send("User not found.")
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionMute, TargetID: &id, Detail: reason}); err != nil {
		h.logWith(ctx).Error("add mute audit log", slog.Any("error", err), slog.Int64("target_id", id))
	}
	return c.Send(fmt.Sprintf("User %d muted.", id))
}

func (h *Handler) handleUnmuteCommand(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	id, _, ok := parseIDReason(c.Text())
	if !ok {
		return c.Send("Usage: /unmute <telegram_id>")
	}
	if err := h.store.SetUserStatus(ctx, id, domain.UserStatusNormal, ""); err != nil {
		return c.Send("User not found.")
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionUnmute, TargetID: &id}); err != nil {
		h.logWith(ctx).Error("add unmute audit log", slog.Any("error", err), slog.Int64("target_id", id))
	}
	return c.Send(fmt.Sprintf("User %d unmuted.", id))
}

func (h *Handler) handleUserCommand(c tele.Context) error {
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		return c.Send("Usage: /user <telegram_id>")
	}
	ctx, cancel := h.requestContext()
	defer cancel()
	user, err := h.store.GetUser(ctx, cmd.UserID)
	if err != nil {
		return c.Send("User not found.")
	}
	return c.Send(formatDomainUserInfo(user), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (h *Handler) handleStats(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	stats, err := h.store.Stats(ctx, time.Now().UTC())
	if err != nil {
		h.logWith(ctx).Error("stats", slog.Any("error", err))
		return c.Send("Stats unavailable.")
	}
	return c.Send(formatDomainStats(stats), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (h *Handler) handleRecent(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	limit := 10
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		h.logWith(ctx).Debug("parse recent command", slog.Any("error", err), slog.String("text", c.Text()))
	}
	if cmd.Limit > 0 {
		limit = min(cmd.Limit, 50)
	}
	users, err := h.store.RecentUsers(ctx, limit)
	if err != nil {
		return c.Send("Recent users unavailable.")
	}
	return c.Send(FormatUserList("Recent users", users), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (h *Handler) handleBlocklist(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	users, err := h.store.BlockedUsers(ctx, 50)
	if err != nil {
		return c.Send("Blocklist unavailable.")
	}
	return c.Send(FormatUserList("Blocked users", users), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (h *Handler) handleAudit(c tele.Context) error {
	ctx, cancel := h.requestContext()
	defer cancel()
	limit := 10
	cmd, err := ParseCommand(c.Text())
	if err != nil {
		h.logWith(ctx).Debug("parse audit command", slog.Any("error", err), slog.String("text", c.Text()))
	}
	if cmd.Limit > 0 {
		limit = min(cmd.Limit, 50)
	}
	logs, err := h.store.GetAuditLogs(ctx, limit)
	if err != nil {
		return c.Send("Audit logs unavailable.")
	}
	return c.Send(FormatAuditLogs(logs), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func ownerHelpText() string {
	return strings.Join([]string{
		"*Owner panel*",
		"",
		"`/stats`",
		"`/recent [limit]`",
		"`/user <telegram_id>`",
		"`/ban <telegram_id> [reason]`",
		"`/unban <telegram_id>`",
		"`/mute <telegram_id> [reason]`",
		"`/unmute <telegram_id>`",
		"`/blocklist`",
		"`/audit [limit]`",
		"`/reply <telegram_id> <message>`",
		"`/cancelreply`",
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

func (h *Handler) refreshExpiredTemporaryBan(ctx context.Context, user *domain.User, now time.Time) error {
	if user.BannedUntil == nil || user.BannedUntil.After(now) {
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

func (h *Handler) requestContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(h.rootCtx, 30*time.Second)
	traceID := generateTraceID()
	ctx = context.WithValue(ctx, contextKeyTraceID, traceID)
	return ctx, cancel
}

func generateTraceID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "000000"
	}
	return hex.EncodeToString(buf[:])
}

func (h *Handler) logWith(ctx context.Context) *slog.Logger {
	if traceID, ok := ctx.Value(contextKeyTraceID).(string); ok && traceID != "" {
		return h.log.With(slog.String("trace_id", traceID))
	}
	return h.log
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
	if err := h.store.AddRateEvent(ctx, telegramID, domain.RateEventTypeAutoBan); err != nil {
		h.log.Error("add auto-ban rate event", slog.Any("error", err), slog.Int64("telegram_id", telegramID))
	}
	if err := h.store.AddAuditLog(ctx, domain.AuditLog{
		ActorID:  h.cfg.OwnerID,
		Action:   domain.AuditActionAutoBan,
		TargetID: &telegramID,
		Detail:   reason,
	}); err != nil {
		h.log.Error("add auto-ban audit log", slog.Any("error", err), slog.Int64("telegram_id", telegramID))
	}
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
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func applyOwnerPrefix(prefix, text string) string {
	if prefix == "" {
		return text
	}
	return prefix + text
}
