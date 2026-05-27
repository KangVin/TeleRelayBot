package store

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	telegram_id INTEGER NOT NULL UNIQUE,
	username TEXT,
	first_name TEXT,
	last_name TEXT,
	language_code TEXT,
	status TEXT NOT NULL DEFAULT 'normal',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	last_seen_at DATETIME,
	message_count INTEGER NOT NULL DEFAULT 0,
	limited_count INTEGER NOT NULL DEFAULT 0,
	ban_reason TEXT,
	banned_until DATETIME
);

CREATE TABLE IF NOT EXISTS message_mappings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_message_id INTEGER NOT NULL,
	owner_chat_id INTEGER NOT NULL,
	stranger_chat_id INTEGER NOT NULL,
	stranger_message_id INTEGER,
	message_type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'open',
	created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_message_mappings_owner_msg
ON message_mappings(owner_chat_id, owner_message_id);

CREATE INDEX IF NOT EXISTS idx_message_mappings_stranger
ON message_mappings(stranger_chat_id);

CREATE INDEX IF NOT EXISTS idx_message_mappings_created
ON message_mappings(created_at);

CREATE TABLE IF NOT EXISTS rate_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	telegram_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_events_user_time
ON rate_events(telegram_id, created_at);

CREATE TABLE IF NOT EXISTS audit_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	actor_id INTEGER NOT NULL,
	action TEXT NOT NULL,
	target_id INTEGER,
	detail TEXT,
	created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_time
ON audit_logs(action, created_at);

CREATE TABLE IF NOT EXISTS owner_reply_sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id INTEGER NOT NULL,
	target_telegram_id INTEGER NOT NULL,
	mapping_id INTEGER,
	created_at DATETIME NOT NULL,
	expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_owner_reply_sessions_owner
ON owner_reply_sessions(owner_id, expires_at);
`
