CREATE TABLE refresh_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_family TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    replaced_by_id TEXT,
    ip_address TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (replaced_by_id) REFERENCES refresh_sessions(id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE INDEX idx_refresh_sessions_user ON refresh_sessions(user_id);
CREATE INDEX idx_refresh_sessions_family ON refresh_sessions(token_family);
CREATE INDEX idx_refresh_sessions_expiry ON refresh_sessions(expires_at);

CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,
    actor_user_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    old_values TEXT,
    new_values TEXT,
    ip_address TEXT,
    success INTEGER NOT NULL CHECK (success IN (0, 1)),
    created_at TEXT NOT NULL,
    FOREIGN KEY (actor_user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id, created_at DESC);
