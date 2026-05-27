package bot

import tele "gopkg.in/telebot.v4"

func (h *Handler) ownerOnly(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if text, rejected := ownerActionRejection(h.cfg.OwnerID, c.Sender(), c.Chat()); rejected {
			return h.rejectOwnerAction(c, text)
		}
		return next(c)
	}
}

func (h *Handler) isOwner(c tele.Context) bool {
	return c.Sender() != nil && c.Sender().ID == h.cfg.OwnerID
}

func (h *Handler) rejectOwnerAction(c tele.Context, text string) error {
	if c.Callback() != nil {
		return c.Respond(&tele.CallbackResponse{Text: text})
	}
	return c.Send(text)
}

func ownerActionRejection(ownerID int64, sender *tele.User, chat *tele.Chat) (string, bool) {
	if sender == nil || sender.ID != ownerID {
		return "Unauthorized.", true
	}
	if chat != nil && chat.Type != tele.ChatPrivate {
		return "Use the private bot chat for owner actions.", true
	}
	return "", false
}
