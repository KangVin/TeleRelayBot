package bot

import (
	"testing"

	"github.com/KangVin/TeleRelayBot/internal/app"
	tele "gopkg.in/telebot.v4"
)

func TestDetectMediaRespectsConfig(t *testing.T) {
	msg := &tele.Message{Photo: &tele.Photo{}, Caption: "hello"}

	disabled := detectMedia(msg, app.Config{})
	if disabled.Type != "photo" || disabled.Allowed {
		t.Fatalf("disabled photo media = %+v", disabled)
	}

	enabledByPhoto := detectMedia(msg, app.Config{AllowPhoto: true})
	if enabledByPhoto.Type != "photo" || !enabledByPhoto.Allowed || enabledByPhoto.Caption != "hello" {
		t.Fatalf("photo enabled media = %+v", enabledByPhoto)
	}

	enabledByGlobal := detectMedia(&tele.Message{Sticker: &tele.Sticker{}}, app.Config{AllowMedia: true})
	if enabledByGlobal.Type != "sticker" || !enabledByGlobal.Allowed {
		t.Fatalf("global enabled sticker = %+v", enabledByGlobal)
	}
}
