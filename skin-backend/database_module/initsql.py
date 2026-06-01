# 数据库初始化 SQL 脚本
# 包含表结构创建及默认配置插入
# 使用 CREATE TABLE IF NOT EXISTS 和 ON CONFLICT 确保幂等性

INIT_SQL = """
-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    preferred_language TEXT DEFAULT 'zh_CN',
    display_name TEXT DEFAULT '',
    is_admin BOOLEAN DEFAULT FALSE,
    avatar_hash TEXT DEFAULT NULL,
    banned_until BIGINT DEFAULT NULL
);

-- 数据库自动迁移：为旧版本 users 表添加列
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='avatar_hash') THEN
        ALTER TABLE users ADD COLUMN avatar_hash TEXT DEFAULT NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='banned_until') THEN
        ALTER TABLE users ADD COLUMN banned_until BIGINT DEFAULT NULL;
    END IF;
END $$;


-- 创建角色表
CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    texture_model TEXT DEFAULT 'default',
    skin_hash TEXT,
    cape_hash TEXT,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- 创建令牌表
CREATE TABLE IF NOT EXISTS tokens (
    access_token TEXT PRIMARY KEY,
    client_token TEXT NOT NULL,
    user_id TEXT NOT NULL,
    profile_id TEXT,
    created_at BIGINT NOT NULL
);

-- 创建站点 refresh token 表（与 Yggdrasil 游戏令牌的 tokens 表无关）
-- 仅存 refresh token 的 SHA-256 哈希；access token 为无状态 JWT，不入库。
-- 旧库升级：本表为新增表，CREATE TABLE IF NOT EXISTS 本身即迁移，无需 ALTER。
CREATE TABLE IF NOT EXISTS site_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- 创建会话表
CREATE TABLE IF NOT EXISTS sessions (
    server_id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL,
    ip TEXT,
    created_at BIGINT NOT NULL
);
 
-- 创建邀请码表
CREATE TABLE IF NOT EXISTS invites (
    code TEXT PRIMARY KEY,
    created_by TEXT,
    used_by TEXT,
    total_uses INTEGER DEFAULT 1,
    used_count INTEGER DEFAULT 0,
    created_at BIGINT,
    note TEXT DEFAULT ''
);

-- 创建设置表
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- 创建用户材质关联表
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

-- 创建皮肤库表
CREATE TABLE IF NOT EXISTS skin_library (
    skin_hash TEXT PRIMARY KEY,
    texture_type TEXT NOT NULL,
    is_public INTEGER DEFAULT 0,
    uploader TEXT,
    model TEXT DEFAULT 'default',
    name TEXT DEFAULT '',
    created_at BIGINT NOT NULL
);

-- 创建外部节点表
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

-- 创建外部节点白名单
CREATE TABLE IF NOT EXISTS whitelisted_users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    endpoint_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(username, endpoint_id),
    FOREIGN KEY(endpoint_id) REFERENCES fallback_endpoints(id) ON DELETE CASCADE
);

-- 创建验证码表
CREATE TABLE IF NOT EXISTS verification_codes (
    email TEXT,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    PRIMARY KEY(email, type)
);

-- ========== 索引 ==========
-- 全部使用 IF NOT EXISTS 保证幂等；这些索引服务于代码中已有的高频查询路径。

-- profiles：按 user_id 查询角色 + 按 id 游标分页（get_profiles_by_user_cursor 等）
CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles (user_id, id);

-- tokens：按用户清理 / 按时间裁剪 / 保留最近 N 个（delete_expired_tokens、delete_surplus_tokens、
-- delete_tokens_by_user 均以 user_id 为前缀，单个复合索引即可覆盖）
CREATE INDEX IF NOT EXISTS idx_tokens_user_created ON tokens (user_id, created_at);
-- tokens：按角色删除（delete_tokens_by_profile）
CREATE INDEX IF NOT EXISTS idx_tokens_profile_id ON tokens (profile_id);

-- site_refresh_tokens：按用户批量撤销（改密/重置/删号）+ 按过期时间清理
CREATE INDEX IF NOT EXISTS idx_site_refresh_user ON site_refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_site_refresh_expires ON site_refresh_tokens (expires_at);

-- user_textures：用户衣柜按 (created_at, hash) 游标分页（get_for_user_cursor）
-- 排序为 created_at DESC, hash DESC，全 DESC 可由升序复合索引反向扫描满足
CREATE INDEX IF NOT EXISTS idx_user_textures_user_created_hash ON user_textures (user_id, created_at, hash);

-- skin_library：公共皮肤库浏览（is_public 过滤 + 时序游标，get_from_library_cursor）
CREATE INDEX IF NOT EXISTS idx_skin_library_public_created_hash ON skin_library (is_public, created_at, skin_hash);
-- skin_library：管理员全量材质列表（无 is_public 过滤的时序游标，list_all_textures_cursor）
CREATE INDEX IF NOT EXISTS idx_skin_library_created_hash ON skin_library (created_at, skin_hash);

-- whitelisted_users：按 endpoint 列出白名单 / 缓存刷新（UNIQUE(username,endpoint_id) 前缀不匹配）
CREATE INDEX IF NOT EXISTS idx_whitelisted_users_endpoint ON whitelisted_users (endpoint_id);

-- 初始化默认设置
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
('smtp_host', 'smtp.example.com'),
('smtp_port', '465'),
('smtp_user', 'user@example.com'),
('smtp_password', 'password'),
('smtp_ssl', 'true'),
('smtp_sender', 'SkinServer <no-reply@example.com>')
ON CONFLICT (key) DO NOTHING;
"""
