-- +goose Up
-- +goose StatementBegin
-- Self-hosted auth (issue #1039), replacing the Supabase GoTrue-backed
-- implementation. Production Postgres already has an `auth` schema owned by
-- GoTrue (restored from Supabase), containing GoTrue's own tables under the
-- same names this migration wants (`auth.users`, `auth.refresh_tokens`,
-- ...). Detect that schema — via `auth.instances`, a GoTrue/Supabase-only
-- table name that can't collide with anything else — and rename it out of
-- the way automatically, so this migration is self-contained: it applies
-- cleanly to a bare dev/CI database (no `auth` schema at all, this block is
-- a no-op) and also to production (renames the legacy schema first, then
-- creates the new one fresh). The renamed schema is never dropped by this
-- migration or by anything else — api/internal/legacyauth copies rows out
-- of it at boot, and it's left in place afterward as a rollback fallback.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'auth' AND table_name = 'instances'
    ) THEN
        ALTER SCHEMA auth RENAME TO auth_gotrue_legacy;
    END IF;
END $$;

CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE IF NOT EXISTS auth.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.totp_factors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    -- Output of crypto.Sealer.Seal — encrypted at rest.
    secret TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unverified'
    CHECK (status IN ('unverified', 'verified')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Only one verified TOTP factor per user; unverified factors are cleaned up
-- on every enroll attempt so there's no need to constrain those.
CREATE UNIQUE INDEX IF NOT EXISTS totp_factors_user_verified_idx
ON auth.totp_factors (user_id) WHERE status = 'verified';

CREATE TABLE IF NOT EXISTS auth.recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recovery_codes_user_id_idx
ON auth.recovery_codes (user_id);

CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    aal TEXT NOT NULL DEFAULT 'aal1' CHECK (aal IN ('aal1', 'aal2')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx
ON auth.refresh_tokens (user_id);

CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Does not restore a renamed `auth_gotrue_legacy` schema back to `auth` —
-- doing that safely requires knowing whether anything already wrote into
-- the new tables being dropped here, which this migration can't know. If a
-- true rollback to GoTrue is ever needed in production, that rename is a
-- manual step: `ALTER SCHEMA auth_gotrue_legacy RENAME TO auth;` after this
-- Down has run.
DROP TABLE IF EXISTS auth.password_reset_tokens;
DROP TABLE IF EXISTS auth.refresh_tokens;
DROP TABLE IF EXISTS auth.recovery_codes;
DROP TABLE IF EXISTS auth.totp_factors;
DROP TABLE IF EXISTS auth.users;
-- +goose StatementEnd
