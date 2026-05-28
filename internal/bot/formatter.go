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

func FormatOwnerMessage(message OwnerMessage) string {
	var b strings.Builder
	b.WriteString("*New message*\n\n")
	fmt.Fprintf(&b, "ID: `%d`\n", message.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", escapeMD(formatUsername(message.Username)))
	fmt.Fprintf(&b, "Name: %s\n", escapeMD(formatName(message.FirstName, message.LastName)))
	if message.LanguageCode != "" {
		fmt.Fprintf(&b, "Language: %s\n", escapeMD(message.LanguageCode))
	}
	if !message.SentAt.IsZero() {
		fmt.Fprintf(&b, "Sent At: %s\n", escapeMD(formatTime(message.SentAt)))
	}
	b.WriteString("\n*Message:*\n")
	b.WriteString(escapeMD(message.Text))
	return b.String()
}

func FormatUserInfo(user UserInfo) string {
	var b strings.Builder
	b.WriteString("*User Info*\n\n")
	fmt.Fprintf(&b, "ID: `%d`\n", user.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", escapeMD(formatUsername(user.Username)))
	fmt.Fprintf(&b, "Name: %s\n", escapeMD(formatName(user.FirstName, user.LastName)))
	fmt.Fprintf(&b, "Language: %s\n", escapeMD(valueOrDash(user.LanguageCode)))
	fmt.Fprintf(&b, "Status: %s\n", escapeMD(valueOrDash(user.Status)))
	fmt.Fprintf(&b, "Messages: %d\n", user.MessageCount)
	fmt.Fprintf(&b, "Limited Count: %d\n", user.LimitedCount)
	fmt.Fprintf(&b, "Created At: %s\n", escapeMD(formatTime(user.CreatedAt)))
	fmt.Fprintf(&b, "Updated At: %s\n", escapeMD(formatTime(user.UpdatedAt)))
	fmt.Fprintf(&b, "Last Seen At: %s\n", escapeMD(formatTime(user.LastSeenAt)))
	fmt.Fprintf(&b, "Ban Reason: %s\n", escapeMD(valueOrDash(user.BanReason)))
	if user.BannedUntil == nil || user.BannedUntil.IsZero() {
		b.WriteString("Banned Until: -\n")
	} else {
		fmt.Fprintf(&b, "Banned Until: %s\n", escapeMD(formatTime(*user.BannedUntil)))
	}
	return b.String()
}

func FormatStats(stats Stats) string {
	return fmt.Sprintf(
		"*Stats*\n\nTotal Users: %d\nNormal Users: %d\nMuted Users: %d\nBlocked Users: %d\nToday Messages: %d\nToday Limited: %d\nToday Replies: %d",
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
	b.WriteString("*New media message*\n\n")
	fmt.Fprintf(&b, "ID: `%d`\n", user.TelegramID)
	fmt.Fprintf(&b, "Username: %s\n", escapeMD(formatUsername(user.Username)))
	fmt.Fprintf(&b, "Name: %s\n", escapeMD(formatName(user.FirstName, user.LastName)))
	if user.LanguageCode != "" {
		fmt.Fprintf(&b, "Language: %s\n", escapeMD(user.LanguageCode))
	}
	fmt.Fprintf(&b, "Message ID: `%d`\n", strangerMessageID)
	fmt.Fprintf(&b, "Type: %s\n", escapeMD(valueOrDash(mediaType)))
	if strings.TrimSpace(caption) != "" {
		b.WriteString("\n*Caption:*\n")
		b.WriteString(escapeMD(caption))
	}
	b.WriteString("\n\nMedia copy follows this message.")
	return b.String()
}

func FormatUserList(title string, users []domain.User) string {
	var b strings.Builder
	b.WriteString("*" + escapeMD(title) + "*")
	if len(users) == 0 {
		b.WriteString("\n\n-")
		return b.String()
	}
	for _, user := range users {
		fmt.Fprintf(&b, "\n\nID: `%d`\nUsername: %s\nName: %s\nStatus: %s\nMessages: %d",
			user.TelegramID,
			escapeMD(formatUsername(user.Username)),
			escapeMD(formatName(user.FirstName, user.LastName)),
			escapeMD(valueOrDash(string(user.Status))),
			user.MessageCount,
		)
	}
	return b.String()
}

func FormatAuditLogs(logs []domain.AuditLog) string {
	var b strings.Builder
	b.WriteString("*Audit Logs*")
	if len(logs) == 0 {
		b.WriteString("\n\n-")
		return b.String()
	}
	for _, log := range logs {
		target := "-"
		if log.TargetID != nil {
			target = fmt.Sprintf("`%d`", *log.TargetID)
		}
		fmt.Fprintf(&b, "\n\n`%s` %s\nActor: `%d`\nTarget: `%s`",
			escapeMD(log.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC")),
			escapeMD(string(log.Action)),
			log.ActorID,
			target,
		)
		if log.Detail != "" {
			fmt.Fprintf(&b, "\nDetail: %s", escapeMD(log.Detail))
		}
	}
	return b.String()
}

func escapeMD(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "`", "\\`", "\\", "\\\\",
	)
	return replacer.Replace(s)
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
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)
	name := strings.TrimSpace(first + " " + last)
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


