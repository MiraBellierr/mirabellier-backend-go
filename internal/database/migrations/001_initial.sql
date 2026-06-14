-- ============================================================
-- Mirabellier Backend (Go) — Fresh Normalized Schema
-- Migration 001
-- ============================================================

PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;   -- 64MB
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 268435456; -- 256MB
PRAGMA foreign_keys = ON;

-- -------------------------------------------
-- Users & Authentication
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    avatar      TEXT,
    banner      TEXT,
    bio         TEXT,
    location    TEXT,
    website     TEXT,
    discord_id  TEXT UNIQUE,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

-- -------------------------------------------
-- Blog Posts
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS posts (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    content           TEXT NOT NULL,
    user_id           TEXT REFERENCES users(id) ON DELETE SET NULL,
    author            TEXT,
    short_description TEXT,
    thumbnail         TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);

CREATE TABLE IF NOT EXISTS comments (
    id          TEXT PRIMARY KEY,
    post_id     TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    parent_id   TEXT REFERENCES comments(id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);

CREATE TABLE IF NOT EXISTS likes (
    post_id        TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    identity_type  TEXT NOT NULL CHECK(identity_type IN ('user', 'anonymous')),
    identity_key   TEXT NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (post_id, identity_type, identity_key)
);

CREATE TABLE IF NOT EXISTS tags (
    name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag     TEXT NOT NULL REFERENCES tags(name) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_post_tags_tag ON post_tags(tag);

-- -------------------------------------------
-- Anime Feed
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS myanimelist_anime_snapshots (
    feed_key    TEXT PRIMARY KEY,
    username    TEXT NOT NULL,
    fetched_at  TEXT NOT NULL,
    payload     TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_mal_snapshots_fetched_at ON myanimelist_anime_snapshots(fetched_at DESC);

-- -------------------------------------------
-- Daily Quotes
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS quote_snapshots (
    recorded_date   TEXT PRIMARY KEY,
    provider        TEXT NOT NULL,
    source_type     TEXT NOT NULL,
    display_date    TEXT,
    published_at    TEXT,
    fetched_at      TEXT NOT NULL,
    fallback_reason TEXT,
    quotes_json     TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_quote_snapshots_fetched_at ON quote_snapshots(fetched_at DESC);

-- -------------------------------------------
-- Guestbook
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS guestbook_entries (
    id          TEXT PRIMARY KEY,
    user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    author      TEXT NOT NULL,
    message     TEXT NOT NULL,
    website     TEXT,
    mood        TEXT NOT NULL DEFAULT 'sparkly'
                CHECK(mood IN ('sparkly','cozy','sleepy','sunny','chaotic')),
    x           INTEGER NOT NULL DEFAULT 0,
    y           INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_guestbook_created_at ON guestbook_entries(created_at DESC);

-- -------------------------------------------
-- Question of the Day
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS daily_questions (
    recordedDate       TEXT PRIMARY KEY,
    prompt              TEXT NOT NULL,
    createdByUserId     TEXT REFERENCES users(id) ON DELETE SET NULL,
    lockedAt            TEXT,
    archivedAt          TEXT,
    discordNotifiedAt   TEXT,
    createdAt           TEXT NOT NULL DEFAULT (datetime('now')),
    updatedAt           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_daily_questions_updatedAt ON daily_questions(updatedAt DESC);

CREATE TABLE IF NOT EXISTS daily_question_answers (
    id              TEXT PRIMARY KEY,
    recordedDate    TEXT NOT NULL REFERENCES daily_questions(recordedDate) ON DELETE CASCADE,
    userId          TEXT REFERENCES users(id) ON DELETE SET NULL,
    guestName       TEXT,
    identityType    TEXT NOT NULL CHECK(identityType IN ('user', 'guest')),
    identityKey     TEXT NOT NULL,
    answer          TEXT NOT NULL,
    createdAt       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_daily_answers_recordedDate ON daily_question_answers(recordedDate, createdAt DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_answers_identity ON daily_question_answers(recordedDate, identityType, identityKey);

-- -------------------------------------------
-- Shrines
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS shrine_pages (
    slug          TEXT PRIMARY KEY,
    path          TEXT NOT NULL UNIQUE,
    title         TEXT NOT NULL,
    description   TEXT,
    excerpt       TEXT,
    image         TEXT,
    image_alt     TEXT,
    schema_type   TEXT DEFAULT 'CollectionPage',
    about_json    TEXT,
    keywords_json TEXT,
    cta_label     TEXT,
    priority      TEXT,
    changefreq    TEXT,
    payload_json  TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_shrine_pages_path ON shrine_pages(path);

-- -------------------------------------------
-- Arena (Game System)
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS arena_profiles (
    user_id                     TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    level                       INTEGER NOT NULL DEFAULT 1,
    xp                          INTEGER NOT NULL DEFAULT 0,
    coins                       INTEGER NOT NULL DEFAULT 0,
    wins                        INTEGER NOT NULL DEFAULT 0,
    losses                      INTEGER NOT NULL DEFAULT 0,
    win_streak                  INTEGER NOT NULL DEFAULT 0,
    hp                          INTEGER NOT NULL DEFAULT 120,
    power                       INTEGER NOT NULL DEFAULT 12,
    guard                       INTEGER NOT NULL DEFAULT 12,
    speed                       INTEGER NOT NULL DEFAULT 10,
    luck                        INTEGER NOT NULL DEFAULT 6,
    lifetime_coins_earned       INTEGER NOT NULL DEFAULT 0,
    selected_card_instance_id   TEXT,
    last_card_draw_date         TEXT,
    daily_card_draw_count       INTEGER NOT NULL DEFAULT 0,
    catalog_version             TEXT NOT NULL DEFAULT 'v2',
    effects_json                TEXT,
    last_fight_at               TEXT,
    created_at                  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at                  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_arena_profiles_level ON arena_profiles(level DESC);
CREATE INDEX IF NOT EXISTS idx_arena_profiles_coins ON arena_profiles(coins DESC);
CREATE INDEX IF NOT EXISTS idx_arena_profiles_updated_at ON arena_profiles(updated_at DESC);

CREATE TABLE IF NOT EXISTS arena_inventory (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id     TEXT NOT NULL,
    quantity    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_arena_inventory_user_id ON arena_inventory(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_arena_inventory_user_item ON arena_inventory(user_id, item_id);

CREATE TABLE IF NOT EXISTS arena_equipment (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot        TEXT NOT NULL CHECK(slot IN ('weapon','armor','charm')),
    item_id     TEXT NOT NULL,
    equipped_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, slot)
);

CREATE INDEX IF NOT EXISTS idx_arena_equipment_user_id ON arena_equipment(user_id);

CREATE TABLE IF NOT EXISTS arena_fights (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    opponent_user_id TEXT,
    result          TEXT NOT NULL CHECK(result IN ('win','loss')),
    rounds_json     TEXT NOT NULL,
    xp_delta        INTEGER NOT NULL DEFAULT 0,
    coin_delta      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_arena_fights_user_id ON arena_fights(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS arena_card_collection (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_instance_id  TEXT NOT NULL,
    card_json         TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_arena_card_collection_user_id ON arena_card_collection(user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_arena_card_collection_user_card ON arena_card_collection(user_id, card_instance_id);

CREATE TABLE IF NOT EXISTS arena_mal_card_pool (
    mal_id      INTEGER PRIMARY KEY,
    title       TEXT NOT NULL,
    url         TEXT NOT NULL,
    image_url   TEXT NOT NULL,
    mean_score  REAL,
    popularity  INTEGER,
    favorites   INTEGER,
    nsfw        TEXT,
    fetched_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_arena_mal_card_pool_fetched_at ON arena_mal_card_pool(fetched_at DESC);

-- -------------------------------------------
-- Schema Migrations Tracking
-- -------------------------------------------

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
