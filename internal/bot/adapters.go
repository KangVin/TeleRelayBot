package bot

import (
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

func formatDomainUserInfo(user *domain.User) string {
	var lastSeen time.Time
	if user.LastSeenAt != nil {
		lastSeen = *user.LastSeenAt
	}
	return FormatUserInfo(UserInfo{
		TelegramID:   user.TelegramID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		Status:       string(user.Status),
		MessageCount: int(user.MessageCount),
		LimitedCount: int(user.LimitedCount),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		LastSeenAt:   lastSeen,
		BanReason:    user.BanReason,
		BannedUntil:  user.BannedUntil,
	})
}

func formatDomainStats(stats *domain.Stats) string {
	return FormatStats(Stats{
		TotalUsers:    int(stats.TotalUsers),
		NormalUsers:   int(stats.NormalUsers),
		MutedUsers:    int(stats.MutedUsers),
		BlockedUsers:  int(stats.BlockedUsers),
		TodayMessages: int(stats.TodayMessages),
		TodayLimited:  int(stats.TodayLimited),
		TodayReplies:  int(stats.TodayReplies),
	})
}
