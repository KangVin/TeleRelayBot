package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

const headerSep = "|-------|-------|"

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
	b.WriteString("# New message\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString(headerSep + "\n")
	fmt.Fprintf(&b, "| **ID** | `%d` |\n", message.TelegramID)
	fmt.Fprintf(&b, "| **Username** | %s |\n", escapeRichTableCell(formatUsername(message.Username)))
	fmt.Fprintf(&b, "| **Name** | %s |\n", escapeRichTableCell(formatName(message.FirstName, message.LastName)))
	if message.LanguageCode != "" {
		fmt.Fprintf(&b, "| **Language** | %s |\n", escapeRichTableCell(message.LanguageCode))
	}
	if !message.SentAt.IsZero() {
		fmt.Fprintf(&b, "| **Sent At** | %s |\n", escapeRichTableCell(formatTime(message.SentAt)))
	}
	b.WriteString("\n**Message:**\n\n")
	b.WriteString(message.Text)
	return b.String()
}

func FormatUserInfo(user UserInfo) string {
	var b strings.Builder
	b.WriteString("# User Info\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString(headerSep + "\n")
	fmt.Fprintf(&b, "| **ID** | `%d` |\n", user.TelegramID)
	fmt.Fprintf(&b, "| **Username** | %s |\n", escapeRichTableCell(formatUsername(user.Username)))
	fmt.Fprintf(&b, "| **Name** | %s |\n", escapeRichTableCell(formatName(user.FirstName, user.LastName)))
	fmt.Fprintf(&b, "| **Language** | %s |\n", escapeRichTableCell(valueOrDash(user.LanguageCode)))
	fmt.Fprintf(&b, "| **Status** | %s |\n", escapeRichTableCell(valueOrDash(user.Status)))
	fmt.Fprintf(&b, "| **Messages** | %d |\n", user.MessageCount)
	fmt.Fprintf(&b, "| **Limited Count** | %d |\n", user.LimitedCount)
	fmt.Fprintf(&b, "| **Created At** | %s |\n", escapeRichTableCell(formatTime(user.CreatedAt)))
	fmt.Fprintf(&b, "| **Updated At** | %s |\n", escapeRichTableCell(formatTime(user.UpdatedAt)))
	fmt.Fprintf(&b, "| **Last Seen At** | %s |\n", escapeRichTableCell(formatTime(user.LastSeenAt)))
	fmt.Fprintf(&b, "| **Ban Reason** | %s |\n", escapeRichTableCell(valueOrDash(user.BanReason)))
	if user.BannedUntil == nil || user.BannedUntil.IsZero() {
		b.WriteString("| **Banned Until** | - |\n")
	} else {
		fmt.Fprintf(&b, "| **Banned Until** | %s |\n", escapeRichTableCell(formatTime(*user.BannedUntil)))
	}
	return b.String()
}

func FormatStats(stats Stats) string {
	var b strings.Builder
	b.WriteString("# Stats\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Total Users | %d |\n", stats.TotalUsers)
	fmt.Fprintf(&b, "| Normal Users | %d |\n", stats.NormalUsers)
	fmt.Fprintf(&b, "| Muted Users | %d |\n", stats.MutedUsers)
	fmt.Fprintf(&b, "| Blocked Users | %d |\n", stats.BlockedUsers)
	fmt.Fprintf(&b, "| Today Messages | %d |\n", stats.TodayMessages)
	fmt.Fprintf(&b, "| Today Limited | %d |\n", stats.TodayLimited)
	fmt.Fprintf(&b, "| Today Replies | %d |\n", stats.TodayReplies)
	return b.String()
}

func FormatOwnerMediaMessage(user *domain.User, strangerMessageID int, mediaType, caption string) string {
	var b strings.Builder
	b.WriteString("# New media message\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString(headerSep + "\n")
	fmt.Fprintf(&b, "| **ID** | `%d` |\n", user.TelegramID)
	fmt.Fprintf(&b, "| **Username** | %s |\n", escapeRichTableCell(formatUsername(user.Username)))
	fmt.Fprintf(&b, "| **Name** | %s |\n", escapeRichTableCell(formatName(user.FirstName, user.LastName)))
	if user.LanguageCode != "" {
		fmt.Fprintf(&b, "| **Language** | %s |\n", escapeRichTableCell(user.LanguageCode))
	}
	fmt.Fprintf(&b, "| **Message ID** | `%d` |\n", strangerMessageID)
	fmt.Fprintf(&b, "| **Type** | %s |\n", escapeRichTableCell(valueOrDash(mediaType)))
	if strings.TrimSpace(caption) != "" {
		b.WriteString("\n**Caption:**\n\n")
		b.WriteString(caption)
	}
	b.WriteString("\n\nMedia copy follows this message.")
	return b.String()
}

func FormatUserList(title string, users []domain.User) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	if len(users) == 0 {
		b.WriteString("None.\n")
		return b.String()
	}
	b.WriteString("| ID | Username | Name | Status | Msgs |\n")
	b.WriteString("|----|----------|------|--------|------|\n")
	for _, user := range users {
		fmt.Fprintf(&b, "| `%d` | %s | %s | %s | %d |\n",
			user.TelegramID,
			escapeRichTableCell(formatUsername(user.Username)),
			escapeRichTableCell(formatName(user.FirstName, user.LastName)),
			escapeRichTableCell(valueOrDash(string(user.Status))),
			user.MessageCount,
		)
	}
	return b.String()
}

func FormatAuditLogs(logs []domain.AuditLog) string {
	var b strings.Builder
	b.WriteString("# Audit Logs\n\n")
	if len(logs) == 0 {
		b.WriteString("None.\n")
		return b.String()
	}
	b.WriteString("| Time | Action | Actor | Target | Detail |\n")
	b.WriteString("|------|--------|-------|--------|--------|\n")
	for _, log := range logs {
		target := "-"
		if log.TargetID != nil {
			target = fmt.Sprintf("`%d`", *log.TargetID)
		}
		detail := valueOrDash(log.Detail)
		fmt.Fprintf(&b, "| %s | %s | `%d` | %s | %s |\n",
			log.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"),
			string(log.Action),
			log.ActorID,
			target,
			escapeRichTableCell(detail),
		)
	}
	return b.String()
}

func escapeRichTableCell(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
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
