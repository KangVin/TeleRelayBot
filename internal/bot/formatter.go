package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

type OwnerMessage struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	Text         string
	SentAt       time.Time
}

type UserInfo struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	Status       string
	MessageCount int
	LimitedCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastSeenAt   time.Time
	BanReason    string
	BannedUntil  *time.Time
}

type Stats struct {
	TotalUsers    int
	NormalUsers   int
	MutedUsers    int
	BlockedUsers  int
	TodayMessages int
	TodayLimited  int
	TodayReplies  int
}

func FormatOwnerMessage(args ...any) string {
	message := ownerMessageFromArgs(args...)
	var b strings.Builder
	b.WriteString("New message\n\n")
	fmt.Fprintf(&b, "ID: %d\n", message.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", formatUsername(message.Username))
	fmt.Fprintf(&b, "Name: %s\n", formatName(message.FirstName, message.LastName))
	if message.LanguageCode != "" {
		fmt.Fprintf(&b, "Language: %s\n", message.LanguageCode)
	}
	if !message.SentAt.IsZero() {
		fmt.Fprintf(&b, "Sent At: %s\n", formatTime(message.SentAt))
	}
	b.WriteString("\nMessage:\n")
	b.WriteString(message.Text)
	return b.String()
}

func FormatUserInfo(input any) string {
	user := userInfoFromAny(input)
	var b strings.Builder
	b.WriteString("User Info\n\n")
	fmt.Fprintf(&b, "ID: %d\n", user.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", formatUsername(user.Username))
	fmt.Fprintf(&b, "Name: %s\n", formatName(user.FirstName, user.LastName))
	fmt.Fprintf(&b, "Language: %s\n", valueOrDash(user.LanguageCode))
	fmt.Fprintf(&b, "Status: %s\n", valueOrDash(user.Status))
	fmt.Fprintf(&b, "Messages: %d\n", user.MessageCount)
	fmt.Fprintf(&b, "Limited Count: %d\n", user.LimitedCount)
	fmt.Fprintf(&b, "Created At: %s\n", formatTime(user.CreatedAt))
	fmt.Fprintf(&b, "Updated At: %s\n", formatTime(user.UpdatedAt))
	fmt.Fprintf(&b, "Last Seen At: %s\n", formatTime(user.LastSeenAt))
	fmt.Fprintf(&b, "Ban Reason: %s\n", valueOrDash(user.BanReason))
	if user.BannedUntil == nil || user.BannedUntil.IsZero() {
		b.WriteString("Banned Until: -")
	} else {
		fmt.Fprintf(&b, "Banned Until: %s", formatTime(*user.BannedUntil))
	}
	return b.String()
}

func FormatStats(input any) string {
	stats := statsFromAny(input)
	return fmt.Sprintf(
		"Stats\n\nTotal Users: %d\nNormal Users: %d\nMuted Users: %d\nBlocked Users: %d\nToday Messages: %d\nToday Limited: %d\nToday Replies: %d",
		stats.TotalUsers,
		stats.NormalUsers,
		stats.MutedUsers,
		stats.BlockedUsers,
		stats.TodayMessages,
		stats.TodayLimited,
		stats.TodayReplies,
	)
}

func FormatOwnerMediaMessage(user *domain.User, strangerMessageID int, mediaType, caption string) string {
	var b strings.Builder
	b.WriteString("New media message\n\n")
	fmt.Fprintf(&b, "ID: %d\n", user.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", formatUsername(user.Username))
	fmt.Fprintf(&b, "Name: %s\n", formatName(user.FirstName, user.LastName))
	if user.LanguageCode != "" {
		fmt.Fprintf(&b, "Language: %s\n", user.LanguageCode)
	}
	fmt.Fprintf(&b, "Message ID: %d\n", strangerMessageID)
	fmt.Fprintf(&b, "Type: %s\n", valueOrDash(mediaType))
	if strings.TrimSpace(caption) != "" {
		b.WriteString("\nCaption:\n")
		b.WriteString(caption)
	}
	b.WriteString("\n\nMedia copy follows this message.")
	return b.String()
}

func FormatUserList(title string, users []domain.User) string {
	var b strings.Builder
	b.WriteString(title)
	if len(users) == 0 {
		b.WriteString("\n\n-")
		return b.String()
	}
	for _, user := range users {
		fmt.Fprintf(&b, "\n\nID: %d\nUsername: %s\nName: %s\nStatus: %s\nMessages: %d",
			user.TelegramID,
			formatUsername(user.Username),
			formatName(user.FirstName, user.LastName),
			valueOrDash(string(user.Status)),
			user.MessageCount,
		)
	}
	return b.String()
}

func ownerMessageFromArgs(args ...any) OwnerMessage {
	if len(args) == 1 {
		if message, ok := args[0].(OwnerMessage); ok {
			return message
		}
	}
	if len(args) == 3 {
		if user, ok := args[0].(*domain.User); ok {
			return OwnerMessage{
				TelegramID:   user.TelegramID,
				Username:     user.Username,
				FirstName:    user.FirstName,
				LastName:     user.LastName,
				LanguageCode: user.LanguageCode,
				Text:         fmt.Sprint(args[2]),
				SentAt:       lastSeenValue(user.LastSeenAt),
			}
		}
		if user, ok := args[0].(domain.User); ok {
			return OwnerMessage{
				TelegramID:   user.TelegramID,
				Username:     user.Username,
				FirstName:    user.FirstName,
				LastName:     user.LastName,
				LanguageCode: user.LanguageCode,
				Text:         fmt.Sprint(args[2]),
				SentAt:       lastSeenValue(user.LastSeenAt),
			}
		}
	}
	return OwnerMessage{}
}

func userInfoFromAny(input any) UserInfo {
	switch user := input.(type) {
	case UserInfo:
		return user
	case *domain.User:
		if user == nil {
			return UserInfo{}
		}
		return userInfoFromDomain(*user)
	case domain.User:
		return userInfoFromDomain(user)
	default:
		return UserInfo{}
	}
}

func userInfoFromDomain(user domain.User) UserInfo {
	messageCount := int(user.MessageCount)
	limitedCount := int(user.LimitedCount)
	return UserInfo{
		TelegramID:   user.TelegramID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		Status:       string(user.Status),
		MessageCount: messageCount,
		LimitedCount: limitedCount,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		LastSeenAt:   lastSeenValue(user.LastSeenAt),
		BanReason:    user.BanReason,
		BannedUntil:  user.BannedUntil,
	}
}

func statsFromAny(input any) Stats {
	switch stats := input.(type) {
	case Stats:
		return stats
	case *domain.Stats:
		if stats == nil {
			return Stats{}
		}
		return statsFromDomain(*stats)
	case domain.Stats:
		return statsFromDomain(stats)
	default:
		return Stats{}
	}
}

func statsFromDomain(stats domain.Stats) Stats {
	return Stats{
		TotalUsers:    int(stats.TotalUsers),
		NormalUsers:   int(stats.NormalUsers),
		MutedUsers:    int(stats.MutedUsers),
		BlockedUsers:  int(stats.BlockedUsers),
		TodayMessages: int(stats.TodayMessages),
		TodayLimited:  int(stats.TodayLimited),
		TodayReplies:  int(stats.TodayReplies),
	}
}

func formatUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "-"
	}
	if strings.HasPrefix(username, "@") {
		return username
	}
	return "@" + username
}

func formatName(first, last string) string {
	name := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
	if name == "" {
		return "-"
	}
	return name
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format("2006-01-02 15:04:05 UTC")
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func lastSeenValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
