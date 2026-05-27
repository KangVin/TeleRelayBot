package bot

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Command
	}{
		{name: "ban", input: "/ban 123 reason", want: Command{Name: CommandBan, UserID: 123, Reason: "reason"}},
		{name: "unban", input: "/unban 123", want: Command{Name: CommandUnban, UserID: 123}},
		{name: "mute", input: "/mute 123 reason", want: Command{Name: CommandMute, UserID: 123, Reason: "reason"}},
		{name: "unmute", input: "/unmute 123", want: Command{Name: CommandUnmute, UserID: 123}},
		{name: "reply", input: "/reply 123 hello world", want: Command{Name: CommandReply, UserID: 123, Message: "hello world"}},
		{name: "user", input: "/user 123", want: Command{Name: CommandUser, UserID: 123}},
		{name: "recent", input: "/recent 20", want: Command{Name: CommandRecent, Limit: 20}},
		{name: "recent default", input: "/recent", want: Command{Name: CommandRecent, Limit: 10}},
		{name: "cancel reply", input: "/cancelreply", want: Command{Name: CommandCancelReply}},
		{name: "bot mention", input: "/ban@relay_bot 123 repeated spam", want: Command{Name: CommandBan, UserID: 123, Reason: "repeated spam"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseCommand() error = %v", err)
			}
			if got.Name != tt.want.Name || got.UserID != tt.want.UserID || got.Reason != tt.want.Reason || got.Message != tt.want.Message || got.Limit != tt.want.Limit {
				t.Fatalf("ParseCommand() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseCommandMissingArgs(t *testing.T) {
	for _, input := range []string{"/ban", "/unban", "/mute", "/unmute", "/reply 123", "/user", "/cancelreply now"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseCommand(input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseCallbackData(t *testing.T) {
	tests := []struct {
		input string
		want  Callback
	}{
		{input: "reply:1024", want: Callback{Action: "reply", ID: 1024}},
		{input: "quick:1024", want: Callback{Action: "quick", ID: 1024}},
		{input: "quick_send:1024:received", want: Callback{Action: "quick_send", ID: 1024, Payload: "received"}},
		{input: "user:123456789", want: Callback{Action: "user", ID: 123456789}},
		{input: "mute:123456789", want: Callback{Action: "mute", ID: 123456789}},
		{input: "unmute:123456789", want: Callback{Action: "unmute", ID: 123456789}},
		{input: "ban:123456789", want: Callback{Action: "ban", ID: 123456789}},
		{input: "ban_confirm:123456789", want: Callback{Action: "ban_confirm", ID: 123456789}},
		{input: "ban_cancel:123456789", want: Callback{Action: "ban_cancel", ID: 123456789}},
		{input: "unban:123456789", want: Callback{Action: "unban", ID: 123456789}},
		{input: "ignore:1024", want: Callback{Action: "ignore", ID: 1024}},
		{input: "cancel_reply:55", want: Callback{Action: "cancel_reply", ID: 55}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseCallbackData(tt.input)
			if err != nil {
				t.Fatalf("ParseCallbackData() error = %v", err)
			}
			if got.Action != tt.want.Action || got.ID != tt.want.ID || got.Payload != tt.want.Payload {
				t.Fatalf("ParseCallbackData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseCallbackDataInvalid(t *testing.T) {
	for _, input := range []string{"", "reply", ":1024", "reply:abc", "reply:0", "reply:1:extra", "quick_send:1024", "quick_send:1024:", "unknown:1"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseCallbackData(input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseCallbackFromTelebotUniqueAndPayload(t *testing.T) {
	tests := []struct {
		name    string
		unique  string
		data    string
		want    Callback
		wantErr bool
	}{
		{name: "payload only", unique: "quick", data: "1024", want: Callback{Action: "quick", ID: 1024}},
		{name: "old quick send payload", unique: "quick_send", data: "1024|received", want: Callback{Action: "quick_send", ID: 1024, Payload: "received"}},
		{name: "new full payload", unique: "quick", data: "quick:1024", want: Callback{Action: "quick", ID: 1024}},
		{name: "new full quick send payload", unique: "quick_send", data: "quick_send:1024:received", want: Callback{Action: "quick_send", ID: 1024, Payload: "received"}},
		{name: "cancel reply payload", unique: "cancel_reply", data: "55", want: Callback{Action: "cancel_reply", ID: 55}},
		{name: "invalid payload only without unique", data: "1024", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCallback(tt.unique, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCallback() error = %v", err)
			}
			if got.Action != tt.want.Action || got.ID != tt.want.ID || got.Payload != tt.want.Payload {
				t.Fatalf("ParseCallback() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
