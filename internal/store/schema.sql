CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS offers (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    location   TEXT NOT NULL,
    category   TEXT NOT NULL,
    amount     TEXT NOT NULL,
    budget     TEXT NOT NULL,
    status     TEXT NOT NULL,
    signal     TEXT NOT NULL,
    supplier   TEXT NOT NULL,
    progress   INTEGER NOT NULL,
    attention  TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS perspectives (
    key             TEXT PRIMARY KEY,
    sort_order      INTEGER NOT NULL,
    label           TEXT NOT NULL,
    title           TEXT NOT NULL,
    subtitle        TEXT NOT NULL,
    quote           TEXT NOT NULL,
    primary_action  TEXT NOT NULL,
    secondary_action TEXT NOT NULL,
    action_name     TEXT NOT NULL,
    stats_json      TEXT NOT NULL,
    workflow_json   TEXT NOT NULL,
    pain_json       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS modules (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sort_order INTEGER NOT NULL,
    name       TEXT NOT NULL,
    body       TEXT NOT NULL,
    impact     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS roadmap (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sort_order INTEGER NOT NULL,
    phase      TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    role       TEXT NOT NULL,
    route      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    reasoning       TEXT NOT NULL,
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);

CREATE TABLE IF NOT EXISTS feedback (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    kind            TEXT NOT NULL,
    theme           TEXT NOT NULL,
    severity        INTEGER NOT NULL,
    verbatim        TEXT NOT NULL,
    requested       TEXT NOT NULL,
    route           TEXT NOT NULL,
    role            TEXT NOT NULL,
    source          TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    status          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_feedback_status ON feedback(status);
CREATE INDEX IF NOT EXISTS idx_feedback_kind ON feedback(kind);
CREATE INDEX IF NOT EXISTS idx_feedback_created_at ON feedback(created_at);

CREATE TABLE IF NOT EXISTS backlog (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    rationale    TEXT NOT NULL,
    theme        TEXT NOT NULL,
    kind         TEXT NOT NULL,
    count        INTEGER NOT NULL,
    avg_severity REAL NOT NULL,
    score        REAL NOT NULL,
    evidence_json TEXT NOT NULL,
    status       TEXT NOT NULL,
    updated_at   INTEGER NOT NULL
);
