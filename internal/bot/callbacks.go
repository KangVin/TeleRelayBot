package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
	tele "gopkg.in/telebot.v4"
)

func (h *Handler) registerCallbacks() {
	h.bot.Handle(&tele.Btn{Unique: "reply"}, h.ownerOnly(h.handleCallbackReply))
	h.bot.Handle(&tele.Btn{Unique: "quick"}, h.ownerOnly(h.handleCallbackQuick))
	h.bot.Handle(&tele.Btn{Unique: "quick_send"}, h.ownerOnly(h.handleCallbackQuickSend))
	h.bot.Handle(&tele.Btn{Unique: "user"}, h.ownerOnly(h.handleCallbackUser))
	h.bot.Handle(&tele.Btn{Unique: "mute"}, h.ownerOnly(h.handleCallbackMute))
	h.bot.Handle(&tele.Btn{Unique: "unmute"}, h.ownerOnly(h.handleCallbackUnmute))
	h.bot.Handle(&tele.Btn{Unique: "ban"}, h.ownerOnly(h.handleCallbackBan))
	h.bot.Handle(&tele.Btn{Unique: "ban_confirm"}, h.ownerOnly(h.handleCallbackBanConfirm))
	h.bot.Handle(&tele.Btn{Unique: "ban_cancel"}, h.ownerOnly(h.handleCallbackBanCancel))
	h.bot.Handle(&tele.Btn{Unique: "unban"}, h.ownerOnly(h.handleCallbackUnban))
	h.bot.Handle(&tele.Btn{Unique: "ignore"}, h.ownerOnly(h.handleCallbackIgnore))
	h.bot.Handle(&tele.Btn{Unique: "cancel_reply"}, h.ownerOnly(h.handleCallbackCancelReply))
}

func (h *Handler) handleCallbackReply(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	mapping, err := h.store.GetMappingByID(context.Background(), data.ID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Mapping not found."})
	}
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	session, err := h.store.CreateOwnerReplySession(context.Background(), domain.OwnerReplySessionCreate{
		OwnerID:          h.cfg.OwnerID,
		TargetTelegramID: mapping.StrangerChatID,
		MappingID:        &mapping.ID,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		h.log.Error("create owner reply session", slog.Any("error", err))
		return c.Respond(&tele.CallbackResponse{Text: "Reply mode failed."})
	}
	if err := c.Respond(); err != nil {
		return err
	}
	return c.Send(
		fmt.Sprintf("Reply mode active for user %d. Send your next text message within 5 minutes.", mapping.StrangerChatID),
		BuildCancelReplySessionKeyboard(session.ID),
	)
}

func (h *Handler) handleCallbackQuick(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	mappingID := data.ID
	if err := c.Respond(); err != nil {
		return err
	}
	return c.Send("Choose a quick reply:", BuildQuickReplyKeyboard(mappingID))
}

func (h *Handler) handleCallbackQuickSend(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil || data.Payload == "" {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	mappingID := data.ID
	template := quickReplyText(data.Payload)
	mapping, err := h.store.GetMappingByID(context.Background(), mappingID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Mapping not found."})
	}
	if _, err := h.sender.SendUser(mapping.StrangerChatID, template); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Send failed."})
	}
	_ = h.store.UpdateMappingStatus(context.Background(), mappingID, domain.MessageMappingStatusReplied)
	targetID := mapping.StrangerChatID
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionQuickReply, TargetID: &targetID})
	h.editCallbackMessage(c, fmt.Sprintf("Quick reply sent to user %d:\n%s", mapping.StrangerChatID, template))
	return c.Respond(&tele.CallbackResponse{Text: "Quick reply sent."})
}

func (h *Handler) handleCallbackUser(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	id := data.ID
	user, err := h.store.GetUser(context.Background(), id)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "User not found."})
	}
	if err := c.Respond(); err != nil {
		return err
	}
	return c.Send(formatDomainUserInfo(user))
}

func (h *Handler) handleCallbackMute(c tele.Context) error {
	return h.setStatusFromCallback(c, domain.UserStatusMuted, "mute", "User muted.")
}

func (h *Handler) handleCallbackUnmute(c tele.Context) error {
	return h.setStatusFromCallback(c, domain.UserStatusNormal, "unmute", "User unmuted.")
}

func (h *Handler) handleCallbackUnban(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	id := data.ID
	if err := h.store.ClearBan(context.Background(), id); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "User not found."})
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionUnban, TargetID: &id})
	h.refreshOwnerMessageKeyboard(c, id, domain.UserStatusNormal)
	return c.Respond(&tele.CallbackResponse{Text: "User unbanned."})
}

func (h *Handler) handleCallbackBan(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	id := data.ID
	if err := c.Respond(); err != nil {
		return err
	}
	return c.Send(fmt.Sprintf("Confirm banning user %d?", id), BuildBanConfirmKeyboard(id))
}

func (h *Handler) handleCallbackBanConfirm(c tele.Context) error {
	if err := h.setStatusFromCallback(c, domain.UserStatusBlocked, "ban", "User banned."); err != nil {
		return err
	}
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err == nil {
		h.editCallbackMessage(c, fmt.Sprintf("User %d banned.", data.ID))
	}
	return nil
}

func (h *Handler) handleCallbackBanCancel(c tele.Context) error {
	h.editCallbackMessage(c, "Ban cancelled.")
	return c.Respond(&tele.CallbackResponse{Text: "Ban cancelled."})
}

func (h *Handler) handleCallbackIgnore(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	id := data.ID
	mapping, err := h.store.GetMappingByID(context.Background(), id)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Mapping not found."})
	}
	if mapping.Status == domain.MessageMappingStatusIgnored {
		if c.Callback().Message != nil {
			if _, err := h.bot.EditReplyMarkup(c.Callback().Message, BuildIgnoredKeyboard(id)); err != nil && !isTelegramMessageNotModified(err) {
				h.log.Error("refresh ignored keyboard", slog.Any("error", err), slog.Int64("mapping_id", id))
			}
		}
		return c.Respond(&tele.CallbackResponse{Text: "Already ignored."})
	}
	if err := h.store.UpdateMappingStatus(context.Background(), id, domain.MessageMappingStatusIgnored); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Ignore failed."})
	}
	targetID := mapping.StrangerChatID
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: domain.AuditActionIgnore, TargetID: &targetID})
	if c.Callback().Message != nil {
		if _, err := h.bot.EditReplyMarkup(c.Callback().Message, BuildIgnoredKeyboard(id)); err != nil && !isTelegramMessageNotModified(err) {
			h.log.Error("set ignored keyboard", slog.Any("error", err), slog.Int64("mapping_id", id))
		}
	}
	return c.Respond(&tele.CallbackResponse{Text: "Ignored."})
}

func (h *Handler) handleCallbackCancelReply(c tele.Context) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	if err := h.store.DeleteOwnerReplySession(context.Background(), data.ID); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Reply mode is not active."})
	}
	h.editCallbackMessage(c, "Reply mode cancelled.")
	return c.Respond(&tele.CallbackResponse{Text: "Reply mode cancelled."})
}

func (h *Handler) setStatusFromCallback(c tele.Context, status domain.UserStatus, action, message string) error {
	data, err := ParseCallback(c.Callback().Unique, c.Callback().Data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid callback."})
	}
	id := data.ID
	if err := h.store.SetUserStatus(context.Background(), id, status, ""); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "User not found."})
	}
	var auditAction domain.AuditAction
	switch action {
	case "ban":
		auditAction = domain.AuditActionBan
	case "mute":
		auditAction = domain.AuditActionMute
	case "unmute":
		auditAction = domain.AuditActionUnmute
	default:
		auditAction = domain.AuditAction(action)
	}
	_ = h.store.AddAuditLog(context.Background(), domain.AuditLog{ActorID: h.cfg.OwnerID, Action: auditAction, TargetID: &id})
	h.refreshOwnerMessageKeyboard(c, id, status)
	return c.Respond(&tele.CallbackResponse{Text: message})
}

func (h *Handler) refreshOwnerMessageKeyboard(c tele.Context, userID int64, status domain.UserStatus) {
	msg := c.Callback().Message
	if msg == nil {
		return
	}
	mapping, err := h.store.GetMappingByOwnerMessage(context.Background(), h.cfg.OwnerID, int64(msg.ID))
	if err != nil {
		if !errors.Is(err, domain.ErrMappingNotFound) {
			h.log.Error("load mapping for keyboard refresh", slog.Any("error", err), slog.Int64("user_id", userID))
		}
		return
	}
	if _, err := h.bot.EditReplyMarkup(msg, BuildOwnerMessageKeyboard(mapping.ID, userID, string(status))); err != nil && !isTelegramMessageNotModified(err) {
		h.log.Error("refresh owner message keyboard", slog.Any("error", err), slog.Int64("mapping_id", mapping.ID), slog.Int64("user_id", userID))
	}
}

func (h *Handler) editCallbackMessage(c tele.Context, text string) {
	msg := c.Callback().Message
	if msg == nil {
		return
	}
	if _, err := h.bot.Edit(msg, text); err != nil && !isTelegramMessageNotModified(err) {
		h.log.Error("edit callback message", slog.Any("error", err), slog.Int("message_id", msg.ID))
	}
}

func isTelegramMessageNotModified(err error) bool {
	return err != nil && strings.Contains(err.Error(), "message is not modified")
}

func quickReplyText(key string) string {
	switch key {
	case "received":
		return "Received. I will review this and reply if needed."
	case "later":
		return "Received. I will reply later."
	case "thanks":
		return "Thanks for the message."
	default:
		return "Received."
	}
}
