package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/envparser"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfigFromEnv(mapEnv(map[string]string{
		"BOT_TOKEN": "123456:xxx",
		"OWNER_ID":  "123456789",
	}))
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}

	if cfg.BotToken != "123456:xxx" {
		t.Fatalf("BotToken = %q", cfg.BotToken)
	}
	if cfg.OwnerID != 123456789 {
		t.Fatalf("OwnerID = %d", cfg.OwnerID)
	}
	if cfg.DatabasePath != "./data/bot.db" {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.AllowMedia || cfg.AllowPhoto || cfg.AllowDocument || cfg.AllowAudio || cfg.AllowVideo || cfg.AllowVoice || cfg.AllowSticker {
		t.Fatalf("media defaults should be false")
	}
	if cfg.RateLimitShortWindow != 10*time.Second || cfg.RateLimitShortMax != 3 {
		t.Fatalf("short rate limit = %s/%d", cfg.RateLimitShortWindow, cfg.RateLimitShortMax)
	}
	if cfg.RateLimitMinuteWindow != time.Minute || cfg.RateLimitMinuteMax != 10 {
		t.Fatalf("minute rate limit = %s/%d", cfg.RateLimitMinuteWindow, cfg.RateLimitMinuteMax)
	}
	if cfg.RateLimitHourWindow != time.Hour || cfg.RateLimitHourMax != 30 {
		t.Fatalf("hour rate limit = %s/%d", cfg.RateLimitHourWindow, cfg.RateLimitHourMax)
	}
	if cfg.AutoBanOnLimitHits != 5 || cfg.AutoBanDuration != 24*time.Hour {
		t.Fatalf("auto ban = %d/%s", cfg.AutoBanOnLimitHits, cfg.AutoBanDuration)
	}
	if cfg.GlobalForwardPerSecond != 5 {
		t.Fatalf("GlobalForwardPerSecond = %d", cfg.GlobalForwardPerSecond)
	}
	if cfg.MaxTextLength != 2000 {
		t.Fatalf("MaxTextLength = %d", cfg.MaxTextLength)
	}
	if cfg.QuickReplyReceived != "Received. I will review this and reply if needed." {
		t.Fatalf("QuickReplyReceived default = %q", cfg.QuickReplyReceived)
	}
	if cfg.QuickReplyLater != "Received. I will reply later." {
		t.Fatalf("QuickReplyLater default = %q", cfg.QuickReplyLater)
	}
	if cfg.QuickReplyThanks != "Thanks for the message." {
		t.Fatalf("QuickReplyThanks default = %q", cfg.QuickReplyThanks)
	}
}

func TestLoadConfigMissingBotToken(t *testing.T) {
	_, err := LoadConfigFromEnv(mapEnv(map[string]string{"OWNER_ID": "123"}))
	if err == nil {
		t.Fatal("expected missing BOT_TOKEN error")
	}
}

func TestLoadConfigMissingOwnerID(t *testing.T) {
	_, err := LoadConfigFromEnv(mapEnv(map[string]string{"BOT_TOKEN": "token"}))
	if err == nil {
		t.Fatal("expected missing OWNER_ID error")
	}
}

func TestLoadConfigInvalidOwnerID(t *testing.T) {
	_, err := LoadConfigFromEnv(mapEnv(map[string]string{
		"BOT_TOKEN": "token",
		"OWNER_ID":  "abc",
	}))
	if err == nil {
		t.Fatal("expected invalid OWNER_ID error")
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := LoadConfigFromEnv(mapEnv(map[string]string{
		"BOT_TOKEN":                        "token",
		"OWNER_ID":                         "42",
		"DATABASE_PATH":                    "/tmp/bot.db",
		"PUBLIC_BOT_USERNAME":              "relay_bot",
		"ALLOW_MEDIA":                      "true",
		"ALLOW_PHOTO":                      "true",
		"RATE_LIMIT_SHORT_WINDOW_SECONDS":  "5",
		"RATE_LIMIT_SHORT_MAX":             "2",
		"RATE_LIMIT_MINUTE_WINDOW_SECONDS": "30",
		"RATE_LIMIT_MINUTE_MAX":            "4",
		"RATE_LIMIT_HOUR_WINDOW_SECONDS":   "600",
		"RATE_LIMIT_HOUR_MAX":              "8",
		"AUTO_BAN_ON_LIMIT_HITS":           "3",
		"AUTO_BAN_DURATION_MINUTES":        "60",
		"GLOBAL_FORWARD_PER_SECOND":        "7",
		"MAX_TEXT_LENGTH":                  "500",
		"OWNER_REPLY_PREFIX":               "[owner]",
		"QUICK_REPLY_RECEIVED":             "Got it",
		"QUICK_REPLY_LATER":                "BRB",
		"QUICK_REPLY_THANKS":               "Thx",
	}))
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}

	if cfg.DatabasePath != "/tmp/bot.db" || cfg.PublicBotUsername != "relay_bot" || cfg.OwnerReplyPrefix != "[owner]" {
		t.Fatalf("string overrides not loaded: %#v", cfg)
	}
	if !cfg.AllowMedia || !cfg.AllowPhoto {
		t.Fatalf("bool overrides not loaded")
	}
	if cfg.RateLimitShortWindow != 5*time.Second || cfg.RateLimitShortMax != 2 {
		t.Fatalf("short override = %s/%d", cfg.RateLimitShortWindow, cfg.RateLimitShortMax)
	}
	if cfg.AutoBanDuration != time.Hour || cfg.GlobalForwardPerSecond != 7 || cfg.MaxTextLength != 500 {
		t.Fatalf("numeric overrides not loaded: %#v", cfg)
	}
	if cfg.QuickReplyReceived != "Got it" || cfg.QuickReplyLater != "BRB" || cfg.QuickReplyThanks != "Thx" {
		t.Fatalf("quick reply overrides not loaded: %#v", cfg)
	}
}

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(`
# comment
BOT_TOKEN="123456:abc"
OWNER_ID=42
DATABASE_PATH=./data/custom.db # inline comment
export MAX_TEXT_LENGTH=1500
`), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := envparser.LoadDotEnv(path)
	if err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if values["BOT_TOKEN"] != "123456:abc" {
		t.Fatalf("BOT_TOKEN = %q", values["BOT_TOKEN"])
	}
	if values["OWNER_ID"] != "42" {
		t.Fatalf("OWNER_ID = %q", values["OWNER_ID"])
	}
	if values["DATABASE_PATH"] != "./data/custom.db" {
		t.Fatalf("DATABASE_PATH = %q", values["DATABASE_PATH"])
	}
	if values["MAX_TEXT_LENGTH"] != "1500" {
		t.Fatalf("MAX_TEXT_LENGTH = %q", values["MAX_TEXT_LENGTH"])
	}
}

func mapEnv(values map[string]string) getenvFunc {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
