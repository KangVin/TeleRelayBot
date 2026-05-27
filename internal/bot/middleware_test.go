package bot

import (
	"testing"

	tele "gopkg.in/telebot.v4"
)

func TestOwnerActionRejection(t *testing.T) {
	tests := []struct {
		name       string
		ownerID    int64
		sender     *tele.User
		chat       *tele.Chat
		wantText   string
		wantReject bool
	}{
		{
			name:       "owner private allowed",
			ownerID:    1,
			sender:     &tele.User{ID: 1},
			chat:       &tele.Chat{Type: tele.ChatPrivate},
			wantReject: false,
		},
		{
			name:       "non owner rejected",
			ownerID:    1,
			sender:     &tele.User{ID: 2},
			chat:       &tele.Chat{Type: tele.ChatPrivate},
			wantText:   "Unauthorized.",
			wantReject: true,
		},
		{
			name:       "owner group rejected",
			ownerID:    1,
			sender:     &tele.User{ID: 1},
			chat:       &tele.Chat{Type: tele.ChatGroup},
			wantText:   "Use the private bot chat for owner actions.",
			wantReject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotReject := ownerActionRejection(tt.ownerID, tt.sender, tt.chat)
			if gotText != tt.wantText || gotReject != tt.wantReject {
				t.Fatalf("ownerActionRejection() = (%q, %v), want (%q, %v)", gotText, gotReject, tt.wantText, tt.wantReject)
			}
		})
	}
}
