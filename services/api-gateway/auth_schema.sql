CREATE TABLE IF NOT EXISTS accounts (
 id TEXT PRIMARY KEY,
 username TEXT NOT NULL UNIQUE,
 display_name TEXT NOT NULL,
 role TEXT NOT NULL CHECK (role IN ('rider','driver')),
 password_hash BYTEA NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS auth_sessions (
 token_hash BYTEA PRIMARY KEY,
 account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS auth_sessions_account ON auth_sessions(account_id);
CREATE INDEX IF NOT EXISTS auth_sessions_expiry ON auth_sessions(expires_at);
CREATE TABLE IF NOT EXISTS auth_rate_limits (
 key TEXT PRIMARY KEY,
 attempts INTEGER NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
