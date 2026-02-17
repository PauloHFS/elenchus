PRAGMA foreign_keys = ON;

-- 1. Tenants & Settings
CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,    -- subdomínio (ex: 'acme')
    name TEXT NOT NULL,
    settings JSON NOT NULL DEFAULT '{}', 
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. RBAC Roles
CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY, -- 'admin', 'user'
    permissions JSON NOT NULL -- ['read', 'write']
);

-- 3. Users
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role_id TEXT NOT NULL REFERENCES roles(id),
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    avatar_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, email)
);

-- (Outras tabelas...)

-- 9. Email Verification
CREATE TABLE IF NOT EXISTS email_verifications (
    email TEXT NOT NULL PRIMARY KEY,
    token TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 4. Sessions (SCS)
CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);

-- 5. Async Jobs (Queue)
CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT REFERENCES tenants(id),
    type TEXT NOT NULL,
    payload JSON NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    idempotency_key TEXT UNIQUE, 
    attempt_count INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    last_error TEXT,
    run_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_run ON jobs(status, run_at);

-- Tabela para garantir que a execução lógica só aconteça uma vez
CREATE TABLE IF NOT EXISTS processed_jobs (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id),
    processed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. Webhooks (Ingestion Audit)
CREATE TABLE IF NOT EXISTS webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,      -- ex: 'stripe', 'github'
    external_id TEXT,         -- ID do evento no provedor externo
    payload JSON NOT NULL,
    headers JSON NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 6. FTS Content
CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    content TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS posts_idx USING fts5(title, content, content='posts', content_rowid='id');
-- (Triggers aqui...)

-- 8. Password Resets
CREATE TABLE IF NOT EXISTS password_resets (
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL PRIMARY KEY,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_password_resets_email ON password_resets(email);
DROP TRIGGER IF EXISTS posts_ai;
CREATE TRIGGER posts_ai AFTER INSERT ON posts BEGIN
  INSERT INTO posts_idx(rowid, title, content) VALUES (new.id, new.title, new.content);
END;
DROP TRIGGER IF EXISTS posts_ad;
CREATE TRIGGER posts_ad AFTER DELETE ON posts BEGIN
  INSERT INTO posts_idx(posts_idx, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
END;
DROP TRIGGER IF EXISTS posts_au;
CREATE TRIGGER posts_au AFTER UPDATE ON posts BEGIN
  INSERT INTO posts_idx(posts_idx, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
  INSERT INTO posts_idx(rowid, title, content) VALUES(new.id, new.title, new.content);
END;
