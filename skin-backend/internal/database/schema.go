package database

const InitSQL = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
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

CREATE TABLE IF NOT EXISTS email_suffix_policy (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    mode TEXT NOT NULL CHECK (mode IN ('disabled', 'allowlist', 'denylist'))
);

INSERT INTO email_suffix_policy (singleton, mode)
VALUES (TRUE, 'disabled')
ON CONFLICT (singleton) DO NOTHING;

CREATE TABLE IF NOT EXISTS email_suffix_rules (
    list_type TEXT NOT NULL CHECK (list_type IN ('allowlist', 'denylist')),
    suffix TEXT NOT NULL CHECK (suffix LIKE '@%'),
    PRIMARY KEY(list_type, suffix)
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
    enable_profile BOOLEAN DEFAULT TRUE,
    enable_hasjoined BOOLEAN DEFAULT TRUE,
    enable_whitelist BOOLEAN DEFAULT FALSE,
    note TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS fallback_skin_domains (
    endpoint_id INTEGER NOT NULL,
    domain TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    PRIMARY KEY(endpoint_id, domain),
    FOREIGN KEY(endpoint_id) REFERENCES fallback_endpoints(id) ON DELETE CASCADE
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

CREATE TABLE IF NOT EXISTS homepage_media (
    id TEXT PRIMARY KEY,
    media_type TEXT NOT NULL CHECK (media_type IN ('image', 'panorama')),
    title TEXT NOT NULL DEFAULT '',
    storage_path TEXT NOT NULL,
    overlay_opacity_light DOUBLE PRECISION NOT NULL DEFAULT 0.45,
    overlay_opacity_dark DOUBLE PRECISION NOT NULL DEFAULT 0.45,
    start_yaw DOUBLE PRECISION NOT NULL DEFAULT 0,
    start_pitch DOUBLE PRECISION NOT NULL DEFAULT 0,
    yaw_speed_dps DOUBLE PRECISION NOT NULL DEFAULT 4,
    pitch_speed_dps DOUBLE PRECISION NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    duration_ms INTEGER NOT NULL DEFAULT 6000,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS enabled_easter_eggs (
    id TEXT PRIMARY KEY,
    sort_order INTEGER NOT NULL,
    enabled_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS notices (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    content_markdown TEXT NOT NULL,
    display_mode TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    link_text TEXT NOT NULL DEFAULT '',
    link_url TEXT NOT NULL DEFAULT '',
    audience TEXT NOT NULL DEFAULT 'users',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    pinned BOOLEAN NOT NULL DEFAULT FALSE,
    dismissible BOOLEAN NOT NULL DEFAULT TRUE,
    starts_at BIGINT DEFAULT NULL,
    ends_at BIGINT DEFAULT NULL,
    created_by TEXT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS notice_receipts (
    notice_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    read_at BIGINT DEFAULT NULL,
    dismissed_at BIGINT DEFAULT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(notice_id, user_id),
    FOREIGN KEY(notice_id) REFERENCES notices(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS notice_targets (
    notice_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(notice_id, user_id),
    FOREIGN KEY(notice_id) REFERENCES notices(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS identity_providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    issuer_url TEXT NOT NULL,
    authorization_endpoint TEXT NOT NULL,
    token_endpoint TEXT NOT NULL,
    userinfo_endpoint TEXT NOT NULL DEFAULT '',
    jwks_uri TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_ciphertext TEXT NOT NULL DEFAULT '',
    scopes TEXT[] NOT NULL DEFAULT ARRAY['openid', 'profile', 'email']::TEXT[],
    adapter TEXT NOT NULL DEFAULT 'generic_oidc' CHECK(adapter IN ('generic_oidc', 'microsoft')),
    icon_url TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    login_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    link_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    registration_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(issuer_url, client_id)
);

CREATE TABLE IF NOT EXISTS external_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    subject TEXT NOT NULL CHECK(subject <> ''),
    label TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_login_at BIGINT,
    UNIQUE(provider_id, subject),
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(provider_id) REFERENCES identity_providers(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS external_identity_credentials (
    identity_id TEXT PRIMARY KEY,
    refresh_token_ciphertext TEXT NOT NULL DEFAULT '',
    granted_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    authorization_status TEXT NOT NULL DEFAULT 'active'
        CHECK(authorization_status IN ('active', 'reauthorization_required')),
    last_refresh_at BIGINT,
    last_refresh_error_at BIGINT,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(identity_id) REFERENCES external_identities(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS official_profile_bindings (
    id TEXT PRIMARY KEY,
    identity_id TEXT NOT NULL,
    profile_id TEXT NOT NULL UNIQUE,
    remote_uuid TEXT NOT NULL,
    remote_name TEXT NOT NULL,
    remote_skin_url TEXT NOT NULL DEFAULT '',
    remote_cape_url TEXT NOT NULL DEFAULT '',
    remote_skin_model TEXT NOT NULL DEFAULT 'default',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_synced_at BIGINT,
    UNIQUE(identity_id, remote_uuid),
    FOREIGN KEY(identity_id) REFERENCES external_identities(id) ON DELETE RESTRICT,
    FOREIGN KEY(profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS permission_subjects (
    id TEXT PRIMARY KEY,
    user_id TEXT UNIQUE,
    kind TEXT NOT NULL CHECK(kind IN ('user', 'client', 'system')),
    status TEXT NOT NULL CHECK(status IN ('active', 'disabled', 'locked')),
    protected BOOLEAN NOT NULL DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS permission_resources (
    id INTEGER PRIMARY KEY CHECK(id > 0 AND id <= 65535),
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS permission_actions (
    id INTEGER PRIMARY KEY CHECK(id > 0 AND id <= 65535),
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS permission_scopes (
    id INTEGER PRIMARY KEY CHECK(id > 0 AND id <= 65535),
    code TEXT NOT NULL UNIQUE,
    resolver_key TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    id BIGINT PRIMARY KEY CHECK(id > 0 AND id < 281474976710656),
    code TEXT NOT NULL UNIQUE,
    resource_id INTEGER NOT NULL,
    action_id INTEGER NOT NULL,
    scope_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    UNIQUE(resource_id, action_id, scope_id),
    CHECK(id = resource_id::BIGINT * 4294967296 + action_id::BIGINT * 65536 + scope_id::BIGINT),
    FOREIGN KEY(resource_id) REFERENCES permission_resources(id) ON DELETE RESTRICT,
    FOREIGN KEY(action_id) REFERENCES permission_actions(id) ON DELETE RESTRICT,
    FOREIGN KEY(scope_id) REFERENCES permission_scopes(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    system_role BOOLEAN NOT NULL DEFAULT FALSE,
    protected BOOLEAN NOT NULL DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(role_id, permission_id),
    FOREIGN KEY(role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS subject_roles (
    subject_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    granted_by_subject_id TEXT,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(subject_id, role_id),
    FOREIGN KEY(subject_id) REFERENCES permission_subjects(id) ON DELETE CASCADE,
    FOREIGN KEY(role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY(granted_by_subject_id) REFERENCES permission_subjects(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS subject_permission_overrides (
    subject_id TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    effect TEXT NOT NULL CHECK(effect IN ('allow', 'deny')),
    granted_by_subject_id TEXT,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(subject_id, permission_id),
    FOREIGN KEY(subject_id) REFERENCES permission_subjects(id) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE,
    FOREIGN KEY(granted_by_subject_id) REFERENCES permission_subjects(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS session_permission_policies (
    session_kind TEXT NOT NULL,
    entrypoint TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(session_kind, entrypoint, permission_id),
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS delegated_clients (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    redirect_uri TEXT NOT NULL DEFAULT '',
    website_url TEXT NOT NULL DEFAULT '',
    client_type TEXT NOT NULL DEFAULT 'confidential' CHECK(client_type IN ('public', 'confidential')),
    secret_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending', 'active', 'rejected', 'disabled')),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(owner_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS delegated_client_permissions (
    client_id TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(client_id, permission_id),
    FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS delegated_permission_grants (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
	 oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked')),
    created_at BIGINT NOT NULL,
    revoked_at BIGINT,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(subject_id) REFERENCES permission_subjects(id) ON DELETE CASCADE,
    FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS delegated_grant_permissions (
    grant_id TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(grant_id, permission_id),
    FOREIGN KEY(grant_id) REFERENCES delegated_permission_grants(id) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code_hash TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    grant_id TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    code_challenge TEXT NOT NULL,
    code_challenge_method TEXT NOT NULL CHECK(code_challenge_method IN ('S256')),
	 oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
	 nonce TEXT NOT NULL DEFAULT '',
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    consumed_at BIGINT,
    FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(grant_id) REFERENCES delegated_permission_grants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS oauth_authorization_code_permissions (
    code_hash TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(code_hash, permission_id),
    FOREIGN KEY(code_hash) REFERENCES oauth_authorization_codes(code_hash) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    grant_id TEXT NOT NULL,
	 oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    revoked_at BIGINT,
    FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(grant_id) REFERENCES delegated_permission_grants(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS oidc_pairwise_subjects (
	client_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	subject TEXT NOT NULL UNIQUE,
	created_at BIGINT NOT NULL,
	PRIMARY KEY(client_id, user_id),
	FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE,
	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS oauth_device_codes (
    device_code_hash TEXT PRIMARY KEY,
    user_code_hash TEXT NOT NULL UNIQUE,
    client_id TEXT NOT NULL,
    user_id TEXT,
    subject_id TEXT,
    status TEXT NOT NULL CHECK(status IN ('pending', 'approved', 'denied', 'consumed', 'expired')),
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    approved_at BIGINT,
    denied_at BIGINT,
    consumed_at BIGINT,
    last_polled_at BIGINT,
    FOREIGN KEY(client_id) REFERENCES delegated_clients(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY(subject_id) REFERENCES permission_subjects(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS oauth_device_code_permissions (
    device_code_hash TEXT NOT NULL,
    permission_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY(device_code_hash, permission_id),
    FOREIGN KEY(device_code_hash) REFERENCES oauth_device_codes(device_code_hash) ON DELETE CASCADE,
    FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS permission_audit_logs (
    id TEXT PRIMARY KEY,
    actor_subject_id TEXT,
    action TEXT NOT NULL,
    target_subject_id TEXT,
    target_role_id TEXT,
    target_permission_id BIGINT,
    target_client_id TEXT,
    target_grant_id TEXT,
    created_at BIGINT NOT NULL,
    FOREIGN KEY(actor_subject_id) REFERENCES permission_subjects(id) ON DELETE SET NULL,
    FOREIGN KEY(target_subject_id) REFERENCES permission_subjects(id) ON DELETE SET NULL,
    FOREIGN KEY(target_role_id) REFERENCES roles(id) ON DELETE SET NULL,
    FOREIGN KEY(target_permission_id) REFERENCES permissions(id) ON DELETE SET NULL,
    FOREIGN KEY(target_client_id) REFERENCES delegated_clients(id) ON DELETE SET NULL,
    FOREIGN KEY(target_grant_id) REFERENCES delegated_permission_grants(id) ON DELETE SET NULL
);

-- Migrations from the Python 2.4.1 schema only.
ALTER TABLE skin_library DROP CONSTRAINT IF EXISTS skin_library_pkey;
ALTER TABLE skin_library ADD CONSTRAINT skin_library_pkey PRIMARY KEY (skin_hash, texture_type);
ALTER TABLE skin_library ADD COLUMN IF NOT EXISTS usage_count BIGINT NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS tokens;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE external_identity_credentials ADD COLUMN IF NOT EXISTS authorization_status TEXT NOT NULL DEFAULT 'active'
    CHECK(authorization_status IN ('active', 'reauthorization_required'));
ALTER TABLE external_identity_credentials ADD COLUMN IF NOT EXISTS last_refresh_at BIGINT;
ALTER TABLE external_identity_credentials ADD COLUMN IF NOT EXISTS last_refresh_error_at BIGINT;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='fallback_endpoints' AND column_name='skin_domains'
    ) THEN
        INSERT INTO fallback_skin_domains (endpoint_id,domain,sort_order)
        SELECT endpoint.id, btrim(item.domain), item.ordinality
        FROM fallback_endpoints endpoint
        CROSS JOIN LATERAL unnest(string_to_array(endpoint.skin_domains, ','))
            WITH ORDINALITY AS item(domain, ordinality)
        WHERE btrim(item.domain) <> ''
        ON CONFLICT (endpoint_id,domain) DO NOTHING;
        ALTER TABLE fallback_endpoints DROP COLUMN skin_domains;
    END IF;
END $$;

UPDATE users SET created_at = 0 WHERE created_at IS NULL;
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
CREATE INDEX IF NOT EXISTS idx_skin_library_public_created_hash ON skin_library (is_public, created_at, skin_hash);
CREATE INDEX IF NOT EXISTS idx_skin_library_created_hash ON skin_library (created_at, skin_hash);
CREATE INDEX IF NOT EXISTS idx_skin_library_public_usage_created_hash ON skin_library (is_public, usage_count DESC, created_at DESC, skin_hash DESC);
CREATE INDEX IF NOT EXISTS idx_whitelisted_users_endpoint ON whitelisted_users (endpoint_id);
CREATE INDEX IF NOT EXISTS idx_fallback_skin_domains_order ON fallback_skin_domains (endpoint_id, sort_order, domain);
CREATE INDEX IF NOT EXISTS idx_homepage_media_public_order ON homepage_media (enabled, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_enabled_easter_eggs_order ON enabled_easter_eggs (sort_order, id);
CREATE INDEX IF NOT EXISTS idx_notices_active ON notices (enabled, audience, pinned, starts_at, ends_at, created_at, id);
CREATE INDEX IF NOT EXISTS idx_notices_cleanup ON notices (ends_at) WHERE ends_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notice_receipts_user ON notice_receipts (user_id, read_at, dismissed_at);
CREATE INDEX IF NOT EXISTS idx_notice_targets_user ON notice_targets (user_id, notice_id);
CREATE INDEX IF NOT EXISTS idx_permission_subjects_user ON permission_subjects (user_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission ON role_permissions (permission_id, role_id);
CREATE INDEX IF NOT EXISTS idx_subject_roles_role ON subject_roles (role_id, subject_id);
CREATE INDEX IF NOT EXISTS idx_subject_permission_overrides_permission ON subject_permission_overrides (permission_id, subject_id);
CREATE INDEX IF NOT EXISTS idx_session_permission_policies_permission ON session_permission_policies (permission_id, session_kind, entrypoint);
CREATE INDEX IF NOT EXISTS idx_delegated_clients_owner ON delegated_clients (owner_user_id);

-- A user and an OAuth client share one logical active authorization. Keep the
-- newest legacy row before installing the constraint, and invalidate database
-- credentials that still point at the superseded grants.
WITH ranked_active_grants AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY user_id, client_id
               ORDER BY created_at DESC, id DESC
           ) AS position
    FROM delegated_permission_grants
    WHERE status = 'active'
)
UPDATE delegated_permission_grants AS g
SET status = 'revoked',
    revoked_at = COALESCE(
        g.revoked_at,
        (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT
    )
FROM ranked_active_grants AS ranked
WHERE g.id = ranked.id AND ranked.position > 1;

UPDATE oauth_refresh_tokens AS token
SET revoked_at = g.revoked_at
FROM delegated_permission_grants AS g
WHERE token.grant_id = g.id
  AND g.status = 'revoked'
  AND token.revoked_at IS NULL;

DELETE FROM oauth_authorization_codes AS code
USING delegated_permission_grants AS g
WHERE code.grant_id = g.id AND g.status = 'revoked';

CREATE UNIQUE INDEX IF NOT EXISTS idx_delegated_permission_grants_active_user_client
    ON delegated_permission_grants (user_id, client_id)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_delegated_permission_grants_user_client ON delegated_permission_grants (user_id, client_id, status);
CREATE INDEX IF NOT EXISTS idx_delegated_permission_grants_active_created ON delegated_permission_grants (created_at, id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_client_user ON oauth_authorization_codes (client_id, user_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_grant_expiry ON oauth_authorization_codes (grant_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_user_client ON oauth_refresh_tokens (user_id, client_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_grant_active_expiry ON oauth_refresh_tokens (grant_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_oidc_pairwise_subjects_user ON oidc_pairwise_subjects (user_id, client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_device_codes_client_status ON oauth_device_codes (client_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_identity_providers_public ON identity_providers (display_order, created_at, id) WHERE enabled=TRUE;
CREATE INDEX IF NOT EXISTS idx_external_identities_user ON external_identities (user_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_official_profile_bindings_identity ON official_profile_bindings (identity_id, created_at, id);

-- Breaking migration: the v2 identity model replaces the one-shot Microsoft import contract.
-- Legacy Microsoft settings are retained here and migrated transactionally by the identity
-- service after configuration and OIDC discovery are available.
ALTER TABLE delegated_permission_grants ADD COLUMN IF NOT EXISTS oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];
ALTER TABLE oauth_authorization_codes ADD COLUMN IF NOT EXISTS oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];
ALTER TABLE oauth_authorization_codes ADD COLUMN IF NOT EXISTS nonce TEXT NOT NULL DEFAULT '';
ALTER TABLE oauth_refresh_tokens ADD COLUMN IF NOT EXISTS oidc_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];
DELETE FROM permissions WHERE code LIKE 'microsoft_import.%';

INSERT INTO settings (key, value) VALUES
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
