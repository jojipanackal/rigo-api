-- Rigo Database Schema v2
-- Run: psql -U rigo_admin -d rigo_db -f database_v2.sql

-- ============================================================
-- DROP EXISTING TABLES (reverse order due to foreign keys)
-- ============================================================
DROP TABLE IF EXISTS session_cards;
DROP TABLE IF EXISTS user_card_progress;
DROP TABLE IF EXISTS user_stats;
DROP TABLE IF EXISTS deck_collaborators;
DROP TABLE IF EXISTS user_deck_mastery;
DROP TABLE IF EXISTS bookmarks;
DROP TABLE IF EXISTS deck_ratings;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS study_sessions;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS decks;
DROP TABLE IF EXISTS users;

-- ============================================================
-- CORE TABLES
-- ============================================================

CREATE TABLE users (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    bio             TEXT,
    subscribers_count INT DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE decks (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    author_id       INT REFERENCES users(id) ON DELETE SET NULL,
    cards_count     INT DEFAULT 0,
    rating          DECIMAL(3,1) DEFAULT 0.0,
    visit_count     INT DEFAULT 0,
    deck_type       VARCHAR(50) NOT NULL DEFAULT 'flashcards',
    tags            TEXT[] NOT NULL DEFAULT '{}'::text[],
    header_image_url TEXT,
    is_public       BOOLEAN DEFAULT false,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Cards: only shared, immutable content lives here.
-- All per-user progress lives in user_card_progress.
CREATE TABLE cards (
    id              SERIAL PRIMARY KEY,
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    front           TEXT NOT NULL,
    back            TEXT NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- PER-USER PROGRESS (fixes the core FSRS bug)
-- Each (user, card) pair has its own independent FSRS state.
-- ============================================================
CREATE TABLE user_card_progress (
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    card_id         INT REFERENCES cards(id) ON DELETE CASCADE,
    -- Review history
    times_reviewed  INT DEFAULT 0,
    times_correct   INT DEFAULT 0,
    last_reviewed   TIMESTAMP WITH TIME ZONE,
    next_review     TIMESTAMP WITH TIME ZONE,
    -- FSRS fields
    stability       DECIMAL(10,4) DEFAULT 0,
    fsrs_difficulty DECIMAL(5,4) DEFAULT 0.3,
    reps            INT DEFAULT 0,
    lapses          INT DEFAULT 0,
    -- FSRS state: 0=New, 1=Learning, 2=Review, 3=Relearning
    state           INT DEFAULT 0,
    PRIMARY KEY (user_id, card_id)
);

-- ============================================================
-- STUDY SESSIONS
-- ============================================================
CREATE TABLE study_sessions (
    id              SERIAL PRIMARY KEY,
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    cards_studied   INT DEFAULT 0,
    correct_count   INT DEFAULT 0,
    started_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at    TIMESTAMP WITH TIME ZONE
);

-- Individual card responses within a session.
-- Enables per-session replay, streak calculation, and leaderboards.
CREATE TABLE session_cards (
    id              SERIAL PRIMARY KEY,
    session_id      INT REFERENCES study_sessions(id) ON DELETE CASCADE,
    card_id         INT REFERENCES cards(id) ON DELETE CASCADE,
    -- FSRS rating: 1=Again, 2=Hard, 3=Good, 4=Easy
    rating          INT CHECK (rating >= 1 AND rating <= 4),
    answered_at     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- SOCIAL FEATURES
-- ============================================================
CREATE TABLE subscriptions (
    subscriber_id   INT REFERENCES users(id) ON DELETE CASCADE,
    creator_id      INT REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (subscriber_id, creator_id)
);

CREATE TABLE deck_ratings (
    id              SERIAL PRIMARY KEY,
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    rating          INT CHECK (rating >= 1 AND rating <= 5),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, deck_id)
);

CREATE TABLE bookmarks (
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, deck_id)
);

-- Per-user mastery percentage for a deck (computed and cached).
CREATE TABLE user_deck_mastery (
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    mastery_percent INT DEFAULT 0 CHECK (mastery_percent >= 0 AND mastery_percent <= 100),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, deck_id)
);

-- ============================================================
-- COLLABORATIVE DECKS
-- Roles: 'editor' can add/edit/delete cards; 'viewer' can only study.
-- ============================================================
CREATE TABLE deck_collaborators (
    deck_id         INT REFERENCES decks(id) ON DELETE CASCADE,
    user_id         INT REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(20) NOT NULL DEFAULT 'editor' CHECK (role IN ('editor', 'viewer')),
    invited_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deck_id, user_id)
);

-- ============================================================
-- STREAKS & LEADERBOARDS
-- Cached stats updated after each completed study session.
-- Avoids expensive aggregation queries on every page load.
-- ============================================================
CREATE TABLE user_stats (
    user_id             INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_cards_studied INT DEFAULT 0,
    total_correct       INT DEFAULT 0,
    total_sessions      INT DEFAULT 0,
    -- Streak tracking
    current_streak      INT DEFAULT 0,   -- consecutive days with at least one session
    longest_streak      INT DEFAULT 0,
    last_study_date     DATE,
    -- XP for leaderboards (award points per correct card, per session, etc.)
    xp                  INT DEFAULT 0,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- FILE STORAGE
-- ============================================================
CREATE TABLE documents (
    id              SERIAL PRIMARY KEY,
    doc_type        VARCHAR(50) NOT NULL,   -- 'deck_icon', 'card_image', etc.
    doc_format      VARCHAR(20) NOT NULL,   -- 'png', 'jpg', 'webp', etc.
    ref_id          INT NOT NULL,           -- deck_id, card_id, etc.
    filename        VARCHAR(255) NOT NULL,
    file_data       BYTEA NOT NULL,
    file_size       INT NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- INDEXES
-- ============================================================
CREATE INDEX idx_cards_deck_id               ON cards(deck_id);
CREATE INDEX idx_decks_author_id             ON decks(author_id);
CREATE INDEX idx_decks_public                ON decks(is_public);
CREATE INDEX idx_decks_visit_count           ON decks(visit_count DESC);
CREATE INDEX idx_study_sessions_user         ON study_sessions(user_id);
CREATE INDEX idx_study_sessions_deck         ON study_sessions(deck_id);
CREATE INDEX idx_session_cards_session       ON session_cards(session_id);
CREATE INDEX idx_ucp_user                    ON user_card_progress(user_id);
-- Critical for the "cards due for review" query
CREATE INDEX idx_ucp_next_review             ON user_card_progress(user_id, next_review);
CREATE INDEX idx_documents_ref               ON documents(doc_type, ref_id);
CREATE INDEX idx_subscriptions_subscriber    ON subscriptions(subscriber_id);
CREATE INDEX idx_subscriptions_creator       ON subscriptions(creator_id);
CREATE INDEX idx_deck_ratings_deck           ON deck_ratings(deck_id);
CREATE INDEX idx_deck_ratings_user           ON deck_ratings(user_id);
CREATE INDEX idx_bookmarks_user              ON bookmarks(user_id);
CREATE INDEX idx_user_deck_mastery_user      ON user_deck_mastery(user_id);
CREATE INDEX idx_deck_collaborators_user     ON deck_collaborators(user_id);
-- For leaderboards: rank users by XP
CREATE INDEX idx_user_stats_xp              ON user_stats(xp DESC);

-- ============================================================
-- SEED DATA
-- ============================================================

-- Demo user (password: "password")
INSERT INTO users (name, email, password_hash)
VALUES ('Demo User', 'demo@rigo.app', '$2a$10$983GgvWTUMAFK4rQ88e6.eNGJiv3hV/25YNPCk3G5YAumZI3Kgpxm');

-- Seed stats row for demo user
INSERT INTO user_stats (user_id) VALUES (1);

-- Sample deck
INSERT INTO decks (name, description, author_id, cards_count, rating, is_public)
VALUES (
    'Golang Basics',
    'Master the fundamentals of Go, including concurrency, interfaces, and the standard library.',
    1, 5, 4.5, false
);

-- Sample cards
INSERT INTO cards (deck_id, front, back) VALUES
(1, 'What is a goroutine?',         'A lightweight thread managed by the Go runtime. Created using the "go" keyword.'),
(1, 'What is a channel in Go?',     'A typed conduit for sending and receiving values between goroutines.'),
(1, 'What does "defer" do?',        'Schedules a function call to run after the surrounding function completes.'),
(1, 'What is an interface in Go?',  'A type that specifies a set of method signatures. Any type implementing those methods satisfies the interface.'),
(1, 'What is the zero value of a slice?', 'nil - a slice with length 0, capacity 0, and no underlying array.');
