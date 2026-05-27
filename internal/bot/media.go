package bot

import (
	"github.com/KangVin/TeleRelayBot/internal/app"
	tele "gopkg.in/telebot.v4"
)

type mediaInfo struct {
	Type    string
	Caption string
	Allowed bool
}

func detectMedia(msg *tele.Message, cfg app.Config) mediaInfo {
	if msg == nil {
		return mediaInfo{}
	}

	info := mediaInfo{Caption: msg.Caption}
	switch {
	case msg.Photo != nil:
		info.Type = "photo"
		info.Allowed = cfg.AllowMedia || cfg.AllowPhoto
	case msg.Document != nil:
		info.Type = "document"
		info.Allowed = cfg.AllowMedia || cfg.AllowDocument
	case msg.Voice != nil:
		info.Type = "voice"
		info.Allowed = cfg.AllowMedia || cfg.AllowVoice
	case msg.Video != nil:
		info.Type = "video"
		info.Allowed = cfg.AllowMedia || cfg.AllowVideo
	case msg.Audio != nil:
		info.Type = "audio"
		info.Allowed = cfg.AllowMedia || cfg.AllowAudio
	case msg.Sticker != nil:
		info.Type = "sticker"
		info.Allowed = cfg.AllowMedia || cfg.AllowSticker
	}
	return info
}
