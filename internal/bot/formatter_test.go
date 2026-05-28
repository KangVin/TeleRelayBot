package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

func TestFormatOwnerMessage(t *testing.T) {
	got := FormatOwnerMessage(OwnerMessage{
		TelegramID:   123456789,
		Username:     "example",
		FirstName:    "Alice",
		LastName:     "Smith",
		LanguageCode: "zh-hans",
		Text:         "hello owner",
		SentAt:       time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
	})

	for _, want := range []string{
		"*New message*",
		"ID: `123456789`",
		"Username: @example",
		"Name: Alice Smith",
		"Language: zh-hans",
		"Sent At: 2026-05-11 12:00:00 UTC",
		"*Message:*\nhello owner",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted owner message missing %q:\n%s", want, got)
		}
	}
}

func TestFormatUserInfo(t *testing.T) {
	bannedUntil := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	got := FormatUserInfo(UserInfo{
		TelegramID:   123456789,
		Username:     "@example",
		FirstName:    "Alice",
		LastName:     "Smith",
		LanguageCode: "zh-hans",
		Status:       "blocked",
		MessageCount: 12,
		LimitedCount: 2,
		CreatedAt:    time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC),
		LastSeenAt:   time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
		BanReason:    "spam",
		BannedUntil:  &bannedUntil,
	})

	for _, want := range []string{
		"*User Info*",
		"ID: `123456789`",
		"Username: @example",
		"Name: Alice Smith",
		"Language: zh-hans",
		"Status: blocked",
		"Messages: 12",
		"Limited Count: 2",
		"Created At: 2026-05-11 10:00:00 UTC",
		"Updated At: 2026-05-11 11:00:00 UTC",
		"Last Seen At: 2026-05-11 12:00:00 UTC",
		"Ban Reason: spam",
		"Banned Until: 2026-05-12 12:00:00 UTC",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted user info missing %q:\n%s", want, got)
		}
	}
}

func TestFormatStats(t *testing.T) {
	got := FormatStats(Stats{
		TotalUsers:    10,
		NormalUsers:   7,
		MutedUsers:    2,
		BlockedUsers:  1,
		TodayMessages: 30,
		TodayLimited:  4,
		TodayReplies:  5,
	})

	for _, want := range []string{
		"Stats",
		"Total Users: 10",
		"Normal Users: 7",
		"Muted Users: 2",
		"Blocked Users: 1",
		"Today Messages: 30",
		"Today Limited: 4",
		"Today Replies: 5",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted stats missing %q:\n%s", want, got)
		}
	}
}

func TestFormatOwnerMediaMessage(t *testing.T) {
	user := &domain.User{
		TelegramID: 123,
		Username:   "alice",
		FirstName:  "Alice",
		Status:     domain.UserStatusNormal,
	}

	got := FormatOwnerMediaMessage(user, 44, "photo", "caption text")
	for _, want := range []string{"*New media message*", "ID: `123`", "Type: photo", "caption text", "Media copy follows"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatOwnerMediaMessage() missing %q in:\n%s", want, got)
		}
	}
}
