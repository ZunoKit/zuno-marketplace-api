BEGIN;

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ======================= USERS =======================
CREATE TABLE IF NOT EXISTS users (
    user_id     uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    status      varchar(32)  NOT NULL DEFAULT 'active',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now()
);

-- Status validation
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_user_status,
  ADD  CONSTRAINT chk_user_status
  CHECK (status IN ('active', 'banned', 'deleted', 'suspended'));

-- Index for status queries
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- ======================= PROFILES =======================
CREATE TABLE IF NOT EXISTS profiles (
    user_id      uuid         PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    username     varchar(30)  UNIQUE,
    display_name varchar(50),
    avatar_url   text,
    banner_url   text,
    bio          text,
    locale       varchar(10)  DEFAULT 'en',
    timezone     varchar(50)  DEFAULT 'UTC',
    socials_json jsonb,
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- Username validation (alphanumeric and underscore only)
ALTER TABLE profiles
  DROP CONSTRAINT IF EXISTS chk_username_format,
  ADD  CONSTRAINT chk_username_format
  CHECK (username ~ '^[a-zA-Z0-9_]{3,30}$');

-- Bio length limit
ALTER TABLE profiles
  DROP CONSTRAINT IF EXISTS chk_bio_length,
  ADD  CONSTRAINT chk_bio_length
  CHECK (char_length(bio) <= 500);

-- Display name length limit
ALTER TABLE profiles
  DROP CONSTRAINT IF EXISTS chk_display_name_length,
  ADD  CONSTRAINT chk_display_name_length
  CHECK (char_length(display_name) <= 50);

-- Indexes for profile queries
CREATE INDEX IF NOT EXISTS idx_profiles_username ON profiles(username);
CREATE INDEX IF NOT EXISTS idx_profiles_updated_at ON profiles(updated_at);

-- ======================= USER PREFERENCES =======================
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id               uuid         PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    email_notifications   boolean      DEFAULT true,
    push_notifications    boolean      DEFAULT true,
    marketing_emails      boolean      DEFAULT false,
    language              varchar(10)  DEFAULT 'en',
    currency              varchar(10)  DEFAULT 'USD',
    theme                 varchar(20)  DEFAULT 'light',
    privacy_level         varchar(20)  DEFAULT 'public',
    show_activity         boolean      DEFAULT true,
    updated_at            timestamptz  NOT NULL DEFAULT now()
);

-- Theme validation
ALTER TABLE user_preferences
  DROP CONSTRAINT IF EXISTS chk_theme,
  ADD  CONSTRAINT chk_theme
  CHECK (theme IN ('light', 'dark', 'auto'));

-- Privacy level validation
ALTER TABLE user_preferences
  DROP CONSTRAINT IF EXISTS chk_privacy_level,
  ADD  CONSTRAINT chk_privacy_level
  CHECK (privacy_level IN ('public', 'private', 'friends'));

-- ======================= USER STATS =======================
CREATE TABLE IF NOT EXISTS user_stats (
    user_id           uuid         PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    collections_count integer      DEFAULT 0,
    items_count       integer      DEFAULT 0,
    listings_count    integer      DEFAULT 0,
    sales_count       integer      DEFAULT 0,
    purchases_count   integer      DEFAULT 0,
    volume_sold       numeric(20,8) DEFAULT 0,
    volume_purchased  numeric(20,8) DEFAULT 0,
    followers_count   integer      DEFAULT 0,
    following_count   integer      DEFAULT 0,
    updated_at        timestamptz  NOT NULL DEFAULT now()
);

-- Non-negative constraints
ALTER TABLE user_stats
  ADD CONSTRAINT chk_stats_non_negative CHECK (
    collections_count >= 0 AND
    items_count >= 0 AND
    listings_count >= 0 AND
    sales_count >= 0 AND
    purchases_count >= 0 AND
    volume_sold >= 0 AND
    volume_purchased >= 0 AND
    followers_count >= 0 AND
    following_count >= 0
  );

-- ======================= USER FOLLOWS =======================
CREATE TABLE IF NOT EXISTS user_follows (
    follower_id  uuid         NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    following_id uuid         NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, following_id)
);

-- Prevent self-follow
ALTER TABLE user_follows
  ADD CONSTRAINT chk_no_self_follow CHECK (follower_id != following_id);

-- Indexes for follow queries
CREATE INDEX IF NOT EXISTS idx_user_follows_follower ON user_follows(follower_id);
CREATE INDEX IF NOT EXISTS idx_user_follows_following ON user_follows(following_id);

-- ======================= FUNCTIONS =======================

-- Update user stats function
CREATE OR REPLACE FUNCTION update_user_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update follower/following counts when follows change
    IF TG_TABLE_NAME = 'user_follows' THEN
        IF TG_OP = 'INSERT' THEN
            UPDATE user_stats SET followers_count = followers_count + 1, updated_at = now() 
            WHERE user_id = NEW.following_id;
            UPDATE user_stats SET following_count = following_count + 1, updated_at = now() 
            WHERE user_id = NEW.follower_id;
        ELSIF TG_OP = 'DELETE' THEN
            UPDATE user_stats SET followers_count = GREATEST(0, followers_count - 1), updated_at = now() 
            WHERE user_id = OLD.following_id;
            UPDATE user_stats SET following_count = GREATEST(0, following_count - 1), updated_at = now() 
            WHERE user_id = OLD.follower_id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for follow stats
CREATE TRIGGER update_follow_stats
    AFTER INSERT OR DELETE ON user_follows
    FOR EACH ROW
    EXECUTE FUNCTION update_user_stats();

-- Auto-create related records when user is created
CREATE OR REPLACE FUNCTION create_user_related_records()
RETURNS TRIGGER AS $$
BEGIN
    -- Create default profile
    INSERT INTO profiles (user_id, username, display_name)
    VALUES (NEW.user_id, NULL, NULL)
    ON CONFLICT (user_id) DO NOTHING;
    
    -- Create default preferences
    INSERT INTO user_preferences (user_id)
    VALUES (NEW.user_id)
    ON CONFLICT (user_id) DO NOTHING;
    
    -- Create default stats
    INSERT INTO user_stats (user_id)
    VALUES (NEW.user_id)
    ON CONFLICT (user_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for creating related records
CREATE TRIGGER create_user_defaults
    AFTER INSERT ON users
    FOR EACH ROW
    EXECUTE FUNCTION create_user_related_records();

-- ======================= COMMENTS =======================
COMMENT ON TABLE users IS 'Core user accounts';
COMMENT ON TABLE profiles IS 'User profile information';
COMMENT ON TABLE user_preferences IS 'User preferences and settings';
COMMENT ON TABLE user_stats IS 'Aggregated user statistics';
COMMENT ON TABLE user_follows IS 'User follow relationships';

COMMIT;
