package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDatabasePath                 = "./data/bot.db"
	defaultRateLimitShortWindowSeconds  = 10
	defaultRateLimitShortMax            = 3
	defaultRateLimitMinuteWindowSeconds = 60
	defaultRateLimitMinuteMax           = 10
	defaultRateLimitHourWindowSeconds   = 3600
	defaultRateLimitHourMax             = 30
	defaultAutoBanOnLimitHits           = 5
	defaultAutoBanDurationMinutes       = 1440
	defaultGlobalForwardPerSecond       = 5
	defaultMaxTextLength                = 2000
)

type Config struct {
	BotToken          string
	OwnerID           int64
	DatabasePath      string
	PublicBotUsername string

	AllowMedia    bool
	AllowPhoto    bool
	AllowDocument bool
	AllowAudio    bool
	AllowVideo    bool
	AllowVoice    bool
	AllowSticker  bool

	RateLimitShortWindow  time.Duration
	RateLimitShortMax     int
	RateLimitMinuteWindow time.Duration
	RateLimitMinuteMax    int
	RateLimitHourWindow   time.Duration
	RateLimitHourMax      int

	AutoBanOnLimitHits     int
	AutoBanDuration        time.Duration
	GlobalForwardPerSecond int
	MaxTextLength          int
	OwnerReplyPrefix       string
}

type getenvFunc func(string) (string, bool)

func LoadConfig() (Config, error) {
	fileEnv, err := loadDotEnv(".env")
	if err != nil {
		return Config{}, err
	}
	return LoadConfigFromEnv(func(key string) (string, bool) {
		if value, ok := os.LookupEnv(key); ok {
			return value, true
		}
		value, ok := fileEnv[key]
		return value, ok
	})
}

func LoadConfigFromEnv(getenv getenvFunc) (Config, error) {
	cfg := Config{
		DatabasePath:           defaultDatabasePath,
		RateLimitShortWindow:   defaultRateLimitShortWindowSeconds * time.Second,
		RateLimitShortMax:      defaultRateLimitShortMax,
		RateLimitMinuteWindow:  defaultRateLimitMinuteWindowSeconds * time.Second,
		RateLimitMinuteMax:     defaultRateLimitMinuteMax,
		RateLimitHourWindow:    defaultRateLimitHourWindowSeconds * time.Second,
		RateLimitHourMax:       defaultRateLimitHourMax,
		AutoBanOnLimitHits:     defaultAutoBanOnLimitHits,
		AutoBanDuration:        defaultAutoBanDurationMinutes * time.Minute,
		GlobalForwardPerSecond: defaultGlobalForwardPerSecond,
		MaxTextLength:          defaultMaxTextLength,
	}

	var err error
	if cfg.BotToken = strings.TrimSpace(getString(getenv, "BOT_TOKEN", "")); cfg.BotToken == "" {
		return Config{}, errors.New("BOT_TOKEN is required")
	}

	ownerIDRaw := strings.TrimSpace(getString(getenv, "OWNER_ID", ""))
	if ownerIDRaw == "" {
		return Config{}, errors.New("OWNER_ID is required")
	}
	cfg.OwnerID, err = strconv.ParseInt(ownerIDRaw, 10, 64)
	if err != nil || cfg.OwnerID <= 0 {
		return Config{}, fmt.Errorf("OWNER_ID must be a positive integer")
	}

	cfg.DatabasePath = getString(getenv, "DATABASE_PATH", cfg.DatabasePath)
	cfg.PublicBotUsername = getString(getenv, "PUBLIC_BOT_USERNAME", "")
	cfg.OwnerReplyPrefix = getString(getenv, "OWNER_REPLY_PREFIX", "")

	if cfg.AllowMedia, err = getBool(getenv, "ALLOW_MEDIA", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowPhoto, err = getBool(getenv, "ALLOW_PHOTO", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowDocument, err = getBool(getenv, "ALLOW_DOCUMENT", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowAudio, err = getBool(getenv, "ALLOW_AUDIO", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowVideo, err = getBool(getenv, "ALLOW_VIDEO", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowVoice, err = getBool(getenv, "ALLOW_VOICE", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowSticker, err = getBool(getenv, "ALLOW_STICKER", false); err != nil {
		return Config{}, err
	}

	if cfg.RateLimitShortWindow, err = getSeconds(getenv, "RATE_LIMIT_SHORT_WINDOW_SECONDS", defaultRateLimitShortWindowSeconds); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitShortMax, err = getPositiveInt(getenv, "RATE_LIMIT_SHORT_MAX", defaultRateLimitShortMax); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitMinuteWindow, err = getSeconds(getenv, "RATE_LIMIT_MINUTE_WINDOW_SECONDS", defaultRateLimitMinuteWindowSeconds); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitMinuteMax, err = getPositiveInt(getenv, "RATE_LIMIT_MINUTE_MAX", defaultRateLimitMinuteMax); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitHourWindow, err = getSeconds(getenv, "RATE_LIMIT_HOUR_WINDOW_SECONDS", defaultRateLimitHourWindowSeconds); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitHourMax, err = getPositiveInt(getenv, "RATE_LIMIT_HOUR_MAX", defaultRateLimitHourMax); err != nil {
		return Config{}, err
	}
	if cfg.AutoBanOnLimitHits, err = getPositiveInt(getenv, "AUTO_BAN_ON_LIMIT_HITS", defaultAutoBanOnLimitHits); err != nil {
		return Config{}, err
	}
	if cfg.AutoBanDuration, err = getMinutes(getenv, "AUTO_BAN_DURATION_MINUTES", defaultAutoBanDurationMinutes); err != nil {
		return Config{}, err
	}
	if cfg.GlobalForwardPerSecond, err = getPositiveInt(getenv, "GLOBAL_FORWARD_PER_SECOND", defaultGlobalForwardPerSecond); err != nil {
		return Config{}, err
	}
	if cfg.MaxTextLength, err = getPositiveInt(getenv, "MAX_TEXT_LENGTH", defaultMaxTextLength); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getString(getenv getenvFunc, key, fallback string) string {
	if value, ok := getenv(key); ok {
		return value
	}
	return fallback
}

func getBool(getenv getenvFunc, key string, fallback bool) (bool, error) {
	raw, ok := getenv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return value, nil
}

func getPositiveInt(getenv getenvFunc, key string, fallback int) (int, error) {
	raw, ok := getenv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func getSeconds(getenv getenvFunc, key string, fallback int) (time.Duration, error) {
	value, err := getPositiveInt(getenv, key, fallback)
	if err != nil {
		return 0, err
	}
	return time.Duration(value) * time.Second, nil
}

func getMinutes(getenv getenvFunc, key string, fallback int) (time.Duration, error) {
	value, err := getPositiveInt(getenv, key, fallback)
	if err != nil {
		return 0, err
	}
	return time.Duration(value) * time.Minute, nil
}

func loadDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		values[key] = parseDotEnvValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return values, nil
}

func parseDotEnvValue(value string) string {
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}
