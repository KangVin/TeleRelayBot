package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type CommandName string

const (
	CommandBan         CommandName = "ban"
	CommandUnban       CommandName = "unban"
	CommandMute        CommandName = "mute"
	CommandUnmute      CommandName = "unmute"
	CommandReply       CommandName = "reply"
	CommandUser        CommandName = "user"
	CommandRecent      CommandName = "recent"
	CommandCancelReply CommandName = "cancelreply"
)

type Command struct {
	Name    CommandName
	UserID  int64
	Reason  string
	Message string
	Limit   int
	Args    []string
}

type Callback struct {
	Action  string
	ID      int64
	Payload string
	Args    []string
}

func ParseCommand(input string) (Command, error) {
	text := strings.TrimSpace(input)
	if text == "" || !strings.HasPrefix(text, "/") {
		return Command{}, errors.New("command must start with /")
	}

	nameToken, rest, _ := strings.Cut(text, " ")
	name := normalizeCommandName(nameToken)
	switch CommandName(name) {
	case CommandBan, CommandMute:
		id, tail, err := parseIDAndTail(rest)
		if err != nil {
			return Command{}, err
		}
		return Command{Name: CommandName(name), UserID: id, Reason: tail, Args: argsFromIDTail(id, tail)}, nil
	case CommandUnban, CommandUnmute, CommandUser:
		id, tail, err := parseIDAndTail(rest)
		if err != nil {
			return Command{}, err
		}
		if tail != "" {
			return Command{}, fmt.Errorf("/%s accepts only telegram_id", name)
		}
		return Command{Name: CommandName(name), UserID: id, Args: []string{strconv.FormatInt(id, 10)}}, nil
	case CommandReply:
		id, tail, err := parseIDAndTail(rest)
		if err != nil {
			return Command{}, err
		}
		if tail == "" {
			return Command{}, errors.New("/reply requires message")
		}
		return Command{Name: CommandReply, UserID: id, Message: tail, Args: argsFromIDTail(id, tail)}, nil
	case CommandRecent:
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return Command{Name: CommandRecent, Limit: 10}, nil
		}
		fields := strings.Fields(rest)
		if len(fields) != 1 {
			return Command{}, errors.New("/recent accepts at most one limit")
		}
		limit, err := strconv.Atoi(fields[0])
		if err != nil || limit <= 0 {
			return Command{}, errors.New("/recent limit must be a positive integer")
		}
		if limit > 50 {
			limit = 50
		}
		return Command{Name: CommandRecent, Limit: limit, Args: []string{strconv.Itoa(limit)}}, nil
	case CommandCancelReply:
		if strings.TrimSpace(rest) != "" {
			return Command{}, errors.New("/cancelreply accepts no arguments")
		}
		return Command{Name: CommandCancelReply}, nil
	default:
		return Command{}, fmt.Errorf("unsupported command %q", name)
	}
}

func ParseCallbackData(data string) (Callback, error) {
	parts := strings.Split(strings.TrimSpace(data), ":")
	if len(parts) < 2 {
		return Callback{}, errors.New("callback data must use action:id format")
	}

	action := strings.TrimSpace(parts[0])
	if action == "" {
		return Callback{}, errors.New("callback action is required")
	}
	id, err := parsePositiveInt64(parts[1], "callback id")
	if err != nil {
		return Callback{}, err
	}

	switch action {
	case "reply", "quick", "user", "mute", "unmute", "ban", "ban_confirm", "ban_cancel", "unban", "ignore", "cancel_reply":
		if len(parts) != 2 {
			return Callback{}, fmt.Errorf("%s callback must use action:id format", action)
		}
		return Callback{Action: action, ID: id, Args: []string{strconv.FormatInt(id, 10)}}, nil
	case "quick_send":
		if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
			return Callback{}, errors.New("quick_send callback must use quick_send:id:key format")
		}
		payload := strings.TrimSpace(parts[2])
		return Callback{Action: action, ID: id, Payload: payload, Args: []string{strconv.FormatInt(id, 10), payload}}, nil
	default:
		return Callback{}, fmt.Errorf("unsupported callback action %q", action)
	}
}

func ParseCallback(unique, data string) (Callback, error) {
	data = strings.TrimSpace(data)
	if strings.Contains(data, ":") {
		return ParseCallbackData(data)
	}

	unique = strings.TrimSpace(unique)
	if unique == "" {
		return ParseCallbackData(data)
	}
	if data == "" {
		return Callback{}, errors.New("callback data is required")
	}
	data = strings.ReplaceAll(data, "|", ":")
	return ParseCallbackData(unique + ":" + data)
}

func (c Command) Int64(index int) (int64, error) {
	if index < 0 || index >= len(c.Args) {
		return 0, errors.New("argument index out of range")
	}
	return parsePositiveInt64(c.Args[index], "argument")
}

func (c Callback) Int64(index int) (int64, error) {
	if index < 0 || index >= len(c.Args) {
		return 0, errors.New("argument index out of range")
	}
	return parsePositiveInt64(c.Args[index], "argument")
}

func normalizeCommandName(token string) string {
	token = strings.TrimPrefix(strings.TrimSpace(token), "/")
	token, _, _ = strings.Cut(token, "@")
	return strings.ToLower(token)
}

func parseIDAndTail(input string) (int64, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, "", errors.New("telegram_id is required")
	}
	idToken, tail, _ := strings.Cut(input, " ")
	id, err := parsePositiveInt64(idToken, "telegram_id")
	if err != nil {
		return 0, "", err
	}
	return id, strings.TrimSpace(tail), nil
}

func parsePositiveInt64(input, label string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return value, nil
}

func argsFromIDTail(id int64, tail string) []string {
	args := []string{strconv.FormatInt(id, 10)}
	if tail == "" {
		return args
	}
	return append(args, strings.Fields(tail)...)
}
