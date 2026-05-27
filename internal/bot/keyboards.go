package bot

import (
	"strconv"

	tele "gopkg.in/telebot.v4"
)

func BuildOwnerMessageKeyboard(mappingID int64, userID int64, status string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	mappingPayload := strconv.FormatInt(mappingID, 10)
	userPayload := strconv.FormatInt(userID, 10)

	reply := markup.Data("Reply", "reply", "reply:"+mappingPayload)
	quick := markup.Data("Quick reply", "quick", "quick:"+mappingPayload)
	user := markup.Data("User info", "user", "user:"+userPayload)
	ignore := markup.Data("Ignore", "ignore", "ignore:"+mappingPayload)

	switch status {
	case "blocked":
		unban := markup.Data("Unban", "unban", "unban:"+userPayload)
		markup.Inline(markup.Row(user, unban), markup.Row(ignore))
	case "muted":
		unmute := markup.Data("Unmute", "unmute", "unmute:"+userPayload)
		ban := markup.Data("Ban", "ban", "ban:"+userPayload)
		markup.Inline(markup.Row(reply, quick), markup.Row(user, unmute), markup.Row(ban, ignore))
	default:
		mute := markup.Data("Mute", "mute", "mute:"+userPayload)
		ban := markup.Data("Ban", "ban", "ban:"+userPayload)
		markup.Inline(markup.Row(reply, quick), markup.Row(user, mute), markup.Row(ban, ignore))
	}

	return markup
}

func BuildQuickReplyKeyboard(mappingID int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	mappingPayload := strconv.FormatInt(mappingID, 10)
	received := markup.Data("Received", "quick_send", "quick_send:"+mappingPayload+":received")
	later := markup.Data("Will reply later", "quick_send", "quick_send:"+mappingPayload+":later")
	thanks := markup.Data("Thanks", "quick_send", "quick_send:"+mappingPayload+":thanks")
	markup.Inline(markup.Row(received), markup.Row(later), markup.Row(thanks))
	return markup
}

func BuildBanConfirmKeyboard(userID int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	userPayload := strconv.FormatInt(userID, 10)
	confirm := markup.Data("Confirm ban", "ban_confirm", "ban_confirm:"+userPayload)
	cancel := markup.Data("Cancel", "ban_cancel", "ban_cancel:"+userPayload)
	markup.Inline(markup.Row(confirm, cancel))
	return markup
}

func BuildCancelReplySessionKeyboard(sessionID int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	sessionPayload := strconv.FormatInt(sessionID, 10)
	cancel := markup.Data("Cancel reply mode", "cancel_reply", "cancel_reply:"+sessionPayload)
	markup.Inline(markup.Row(cancel))
	return markup
}

func BuildIgnoredKeyboard(mappingID int64) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	mappingPayload := strconv.FormatInt(mappingID, 10)
	ignored := markup.Data("Ignored", "ignore", "ignore:"+mappingPayload)
	markup.Inline(markup.Row(ignored))
	return markup
}
