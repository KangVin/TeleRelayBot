package bot

import (
	"testing"

	tele "gopkg.in/telebot.v4"
)

func TestBuildOwnerMessageKeyboardCallbackPayloads(t *testing.T) {
	markup := BuildOwnerMessageKeyboard(1024, 123456789, "normal")

	assertButton(t, markup, 0, 0, "reply", "reply:1024")
	assertButton(t, markup, 0, 1, "quick", "quick:1024")
	assertButton(t, markup, 1, 0, "user", "user:123456789")
	assertButton(t, markup, 1, 1, "mute", "mute:123456789")
	assertButton(t, markup, 2, 0, "ban", "ban:123456789")
	assertButton(t, markup, 2, 1, "ignore", "ignore:1024")
}

func TestBuildQuickReplyKeyboardCallbackPayloads(t *testing.T) {
	markup := BuildQuickReplyKeyboard(1024)

	assertButton(t, markup, 0, 0, "quick_send", "quick_send:1024:received")
	assertButton(t, markup, 1, 0, "quick_send", "quick_send:1024:later")
	assertButton(t, markup, 2, 0, "quick_send", "quick_send:1024:thanks")
}

func TestBuildBanConfirmKeyboardCallbackPayloads(t *testing.T) {
	markup := BuildBanConfirmKeyboard(123456789)

	assertButton(t, markup, 0, 0, "ban_confirm", "ban_confirm:123456789")
	assertButton(t, markup, 0, 1, "ban_cancel", "ban_cancel:123456789")
}

func TestBuildCancelReplySessionKeyboardCallbackPayloads(t *testing.T) {
	markup := BuildCancelReplySessionKeyboard(55)

	assertButton(t, markup, 0, 0, "cancel_reply", "cancel_reply:55")
}

func TestBuildIgnoredKeyboardCallbackPayloads(t *testing.T) {
	markup := BuildIgnoredKeyboard(1024)

	assertButton(t, markup, 0, 0, "ignore", "ignore:1024")
	if markup.InlineKeyboard[0][0].Text != "Ignored" {
		t.Fatalf("ignored button text = %q", markup.InlineKeyboard[0][0].Text)
	}
}

func assertButton(t *testing.T, markup *tele.ReplyMarkup, row, col int, unique, data string) {
	t.Helper()
	keyboard := markup.InlineKeyboard
	if len(keyboard) <= row || len(keyboard[row]) <= col {
		t.Fatalf("missing button at row %d col %d", row, col)
	}
	button := keyboard[row][col]
	if button.Unique != unique || button.Data != data {
		t.Fatalf("button row %d col %d = unique %q data %q", row, col, button.Unique, button.Data)
	}
}
