# Telegram Relay Bot

A single-owner Telegram private-message relay bot written in Go. Strangers message the bot, the bot forwards formatted messages to the owner, and the owner replies through the bot without exposing the owner's Telegram account.

## Features

- Long polling with `gopkg.in/telebot.v4`
- Text relay from strangers to one owner
- Owner replies by Telegram reply, button reply mode, or `/reply`
- Inline buttons for reply, quick replies, user info, mute, ban, unban, and ignore
- SQLite persistence with `modernc.org/sqlite`
- Per-user rate limits, global owner-forward throttling, and temporary auto-ban
- Owner-only admin commands and callbacks
- Structured logs through `log/slog`

## Configuration

Copy `.env.example` to `.env`, or provide the same keys through your service manager. The app loads `.env` from the current working directory, then real environment variables override `.env` values.

| Key | Required | Default | Description |
| --- | --- | --- | --- |
| `BOT_TOKEN` | Yes | | Telegram bot token from `@BotFather`. |
| `OWNER_ID` | Yes | | Numeric Telegram user ID allowed to administer the bot. |
| `DATABASE_PATH` | No | `./data/bot.db` | SQLite database path. Use `/app/data/bot.db` in Docker. |
| `PUBLIC_BOT_USERNAME` | No | | Public username used by command parsing when commands include `@botname`. |
| `ALLOW_MEDIA` | No | `false` | Enables all supported media relay types. |
| `ALLOW_PHOTO` / `ALLOW_DOCUMENT` / `ALLOW_AUDIO` / `ALLOW_VIDEO` / `ALLOW_VOICE` / `ALLOW_STICKER` | No | `false` | Enables individual media relay types. |
| `RATE_LIMIT_SHORT_WINDOW_SECONDS` | No | `10` | Short rate-limit window length. |
| `RATE_LIMIT_SHORT_MAX` | No | `3` | Max messages in the short window. |
| `RATE_LIMIT_MINUTE_WINDOW_SECONDS` | No | `60` | Minute rate-limit window length. |
| `RATE_LIMIT_MINUTE_MAX` | No | `10` | Max messages in the minute window. |
| `RATE_LIMIT_HOUR_WINDOW_SECONDS` | No | `3600` | Hour rate-limit window length. |
| `RATE_LIMIT_HOUR_MAX` | No | `30` | Max messages in the hour window. |
| `AUTO_BAN_ON_LIMIT_HITS` | No | `5` | Temporary-ban threshold after repeated rate-limit hits. |
| `AUTO_BAN_DURATION_MINUTES` | No | `1440` | Temporary auto-ban duration. |
| `GLOBAL_FORWARD_PER_SECOND` | No | `5` | Global throttle for messages forwarded to the owner. |
| `MAX_TEXT_LENGTH` | No | `2000` | Max inbound or reply text length. |
| `OWNER_REPLY_PREFIX` | No | | Optional prefix prepended to owner replies. |
| `QUICK_REPLY_RECEIVED` | No | `Received. I will review this and reply if needed.` | Custom quick-reply template for "received". |
| `QUICK_REPLY_LATER` | No | `Received. I will reply later.` | Custom quick-reply template for "reply later". |
| `QUICK_REPLY_THANKS` | No | `Thanks for the message.` | Custom quick-reply template for "thanks". |

`.env` supports `KEY=VALUE`, optional `export`, simple single or double quotes, and inline comments. Inline comments (` # text`) are only stripped from unquoted values; quoted values preserve ` #` as-is, for example `PASSWORD="pass #123"`.

## Local Run

```bash
cp .env.example .env
go run ./cmd/bot
```

Build and test:

```bash
go build -o telegram-relay-bot ./cmd/bot
go test ./...
```

## Docker

The Dockerfile is a production multi-stage build. It builds a static Linux binary with `CGO_ENABLED=0`, which is compatible with the pure-Go `modernc.org/sqlite` driver used by this project. Runtime state is stored in `/app/data`; mount it or you will lose the SQLite database when the container is replaced.

```bash
docker build -t telegram-relay-bot .
docker run -d --name telegram-relay-bot \
  --restart unless-stopped \
  --env-file .env \
  -e DATABASE_PATH=/app/data/bot.db \
  -v telegram-relay-bot-data:/app/data \
  telegram-relay-bot
```

Using a host directory instead:

```bash
mkdir -p data
docker run -d --name telegram-relay-bot \
  --env-file .env \
  -e DATABASE_PATH=/app/data/bot.db \
  -v "$PWD/data:/app/data" \
  telegram-relay-bot
```

## systemd

Install the built binary and an `.env` file under `/opt/telegram-relay-bot`. Keep `DATABASE_PATH` inside that directory or another backed-up path.

```ini
[Unit]
Description=Telegram Relay Bot
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=telegram-relay-bot
Group=telegram-relay-bot
WorkingDirectory=/opt/telegram-relay-bot
EnvironmentFile=/opt/telegram-relay-bot/.env
ExecStart=/opt/telegram-relay-bot/telegram-relay-bot
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Owner Commands

- `/help`: show owner command reference.
- `/stats`: show aggregate user/message stats.
- `/recent [limit]`: list recent users; defaults to 10 and caps at 50.
- `/user <telegram_id>`: show stored user details.
- `/ban <telegram_id> [reason]`: block a user.
- `/unban <telegram_id>`: unblock a user.
- `/mute <telegram_id> [reason]`: accept but stop forwarding a user's messages.
- `/unmute <telegram_id>`: restore a muted user.
- `/blocklist`: list blocked users.
- `/audit [limit]`: show recent audit log entries; defaults to 10 and caps at 50.
- `/reply <telegram_id> <message>`: send a bot message to a user.
- `/cancelreply`: cancel the active button reply session.

Non-owner users receive only a generic unauthorized response for owner-only actions.

## Button Workflow

When a stranger sends text or enabled media in a private chat, the owner receives a formatted relay message with buttons:

- `Reply`: starts a one-message reply mode for that user; the next owner text or enabled media is sent to the user.
- `Quick reply`: opens configurable canned replies (`QUICK_REPLY_*` env vars).
- `User info`: shows the stored user profile and status.
- `Mute` / `Unmute`: toggles forwarding while still acknowledging the user.
- `Ban` / `Unban`: blocks or restores the user; `Ban` asks for confirmation.
- `Ignore`: marks the relayed message ignored without contacting the user.

The owner can also reply directly to a relayed Telegram message instead of using buttons.

## Security Notes

- Never commit `.env`, `BOT_TOKEN`, or the SQLite database.
- Only `OWNER_ID` can run admin commands and button callbacks.
- The owner replies as the bot, not from the owner's Telegram account.
- Stranger intake is limited to private chats; group chats are ignored.
- Persist and back up `/app/data` or the configured `DATABASE_PATH`.
- Keep the host, container runtime, and bot token access restricted.

## Troubleshooting

- `BOT_TOKEN is required` or `OWNER_ID is required`: check the working directory for `.env`, or set real environment variables.
- `OWNER_ID must be a positive integer`: use the numeric Telegram user ID, not a username.
- Database errors on startup: ensure the parent directory of `DATABASE_PATH` exists and is writable by the process or container user.
- Container starts with an empty history: verify that `/app/data` is mounted and `DATABASE_PATH=/app/data/bot.db`.
- Owner does not receive messages: confirm the bot token, that the owner has started the bot, and that rate limits are not being hit.
- Media is rejected: set `ALLOW_MEDIA=true` or the specific `ALLOW_*` key for that media type, then restart the bot.
