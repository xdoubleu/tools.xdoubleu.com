-- +goose Up
-- +goose StatementBegin
-- The latest verified, judged picks per user; each generation replaces them.
CREATE TABLE IF NOT EXISTS taste.recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    domain TEXT NOT NULL,
    track TEXT NOT NULL CHECK (track IN ('more', 'broaden')),
    title TEXT NOT NULL,
    creator TEXT NOT NULL DEFAULT '',
    year INT NOT NULL DEFAULT 0,
    catalog_ref TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    why TEXT NOT NULL DEFAULT '',
    sources JSONB NOT NULL DEFAULT '[]',
    score INT NOT NULL DEFAULT 0,
    generated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_recommendations_user
ON taste.recommendations (user_id, domain, track, score DESC);

-- New columns, so no backfill: no recommendations exist yet.
ALTER TABLE taste.profiles
ADD COLUMN IF NOT EXISTS recommended_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS recommendations_error TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE taste.profiles
DROP COLUMN IF EXISTS recommendations_error,
DROP COLUMN IF EXISTS recommended_at;
DROP TABLE IF EXISTS taste.recommendations;
-- +goose StatementEnd
