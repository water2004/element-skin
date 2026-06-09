package database

const InitSQL = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
    is_admin BOOLEAN DEFAULT FALSE,
    is_super_admin BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL DEFAULT 0,
    avatar_hash TEXT DEFAULT NULL,
    banned_until BIGINT DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    texture_model TEXT DEFAULT 'default',
    skin_hash TEXT,
    cape_hash TEXT,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS site_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id)
);
 
CREATE TABLE IF NOT EXISTS invites (
    code TEXT PRIMARY KEY,
    created_by TEXT,
    used_by TEXT,
    total_uses INTEGER DEFAULT 1,
    used_count INTEGER DEFAULT 0,
    created_at BIGINT,
    note TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS user_textures (
    user_id TEXT NOT NULL,
    hash TEXT NOT NULL,
    texture_type TEXT NOT NULL,
    note TEXT DEFAULT '',
    model TEXT DEFAULT 'default',
    is_public INTEGER DEFAULT 0,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(user_id, hash, texture_type),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS skin_library (
    skin_hash TEXT NOT NULL,
    texture_type TEXT NOT NULL,
    is_public INTEGER DEFAULT 0,
    uploader TEXT,
    model TEXT DEFAULT 'default',
    name TEXT DEFAULT '',
    created_at BIGINT NOT NULL,
    usage_count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY(skin_hash, texture_type)
);

CREATE TABLE IF NOT EXISTS fallback_endpoints (
    id SERIAL PRIMARY KEY,
    priority INTEGER NOT NULL,
    session_url TEXT NOT NULL,
    account_url TEXT NOT NULL,
    services_url TEXT NOT NULL,
    cache_ttl INTEGER NOT NULL,
    skin_domains TEXT DEFAULT '',
    enable_profile BOOLEAN DEFAULT TRUE,
    enable_hasjoined BOOLEAN DEFAULT TRUE,
    enable_whitelist BOOLEAN DEFAULT FALSE,
    note TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS whitelisted_users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    endpoint_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(username, endpoint_id),
    FOREIGN KEY(endpoint_id) REFERENCES fallback_endpoints(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verification_codes (
    email TEXT,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    PRIMARY KEY(email, type)
);

ALTER TABLE skin_library DROP CONSTRAINT IF EXISTS skin_library_pkey;
ALTER TABLE skin_library ADD CONSTRAINT skin_library_pkey PRIMARY KEY (skin_hash, texture_type);
ALTER TABLE skin_library ADD COLUMN IF NOT EXISTS usage_count BIGINT NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS tokens;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_super_admin BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at BIGINT NOT NULL DEFAULT 0;
UPDATE users SET created_at = 0 WHERE created_at IS NULL;
WITH first_user AS (
    SELECT id FROM users ORDER BY created_at ASC, id ASC LIMIT 1
),
super_admin_seed AS (
    SELECT COALESCE(
        (SELECT id FROM users WHERE is_admin = TRUE ORDER BY created_at ASC, id ASC LIMIT 1),
        (SELECT id FROM first_user)
    ) AS id
    WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_super_admin = TRUE)
)
UPDATE users
SET is_super_admin = TRUE,
    is_admin = TRUE
WHERE id = (SELECT id FROM super_admin_seed);
WITH chosen_super_admin AS (
    SELECT id FROM users WHERE is_super_admin = TRUE ORDER BY created_at ASC, id ASC LIMIT 1
)
UPDATE users
SET is_super_admin = (id = (SELECT id FROM chosen_super_admin)),
    is_admin = CASE WHEN id = (SELECT id FROM chosen_super_admin) THEN TRUE ELSE is_admin END
WHERE EXISTS (SELECT 1 FROM chosen_super_admin);
UPDATE skin_library sl SET usage_count = CASE sl.texture_type
    WHEN 'skin' THEN (SELECT COUNT(*) FROM user_textures ut WHERE ut.hash = sl.skin_hash AND ut.texture_type = 'skin')
    WHEN 'cape' THEN (SELECT COUNT(*) FROM user_textures ut WHERE ut.hash = sl.skin_hash AND ut.texture_type = 'cape')
    ELSE (SELECT COUNT(*) FROM user_textures ut WHERE ut.hash = sl.skin_hash AND ut.texture_type = sl.texture_type)
END;

CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles (user_id, id);
CREATE INDEX IF NOT EXISTS idx_site_refresh_user ON site_refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_site_refresh_expires ON site_refresh_tokens (expires_at);
CREATE INDEX IF NOT EXISTS idx_user_textures_user_created_hash ON user_textures (user_id, created_at, hash);
CREATE INDEX IF NOT EXISTS idx_user_textures_hash_type ON user_textures (hash, texture_type);
CREATE INDEX IF NOT EXISTS idx_users_display_name ON users (display_name);
CREATE INDEX IF NOT EXISTS idx_users_created_id ON users (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_super_admin ON users ((is_super_admin)) WHERE is_super_admin = TRUE;
CREATE INDEX IF NOT EXISTS idx_skin_library_public_created_hash ON skin_library (is_public, created_at, skin_hash);
CREATE INDEX IF NOT EXISTS idx_skin_library_created_hash ON skin_library (created_at, skin_hash);
CREATE INDEX IF NOT EXISTS idx_skin_library_public_usage_created_hash ON skin_library (is_public, usage_count DESC, created_at DESC, skin_hash DESC);
CREATE INDEX IF NOT EXISTS idx_whitelisted_users_endpoint ON whitelisted_users (endpoint_id);

INSERT INTO settings (key, value) VALUES
('microsoft_client_id', ''),
('microsoft_client_secret', ''),
('microsoft_redirect_uri', 'http://localhost:8000/microsoft/callback'),
('fallback_strategy', 'serial'),
('profile_uuid_mode', 'random'),
('enable_skin_library', 'true'),
('email_verify_enabled', 'false'),
('enable_strong_password_check', 'false'),
('email_verify_ttl', '300'),
('allow_register', 'true'),
('require_invite', 'false'),
('jwt_expire_days', '7'),
('site_name', '皮肤站'),
('smtp_host', 'smtp.example.com'),
('smtp_port', '465'),
('smtp_user', 'user@example.com'),
('smtp_password', 'password'),
('smtp_ssl', 'true'),
('smtp_sender', 'SkinServer <no-reply@example.com>')
ON CONFLICT (key) DO NOTHING;
`
