-- WebAuthn credentials and their short-lived registration challenges, plus the
-- time of the sign-in a refresh-token chain stems from, so adding or removing
-- a passkey can demand a recent sign-in. Existing refresh tokens take their
-- creation time as auth_time (the last rotation), so none counts as recent
-- longer than it has been idle.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth.passkeys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    attestation_type TEXT NOT NULL,
    aaguid BYTEA NOT NULL,
    flags SMALLINT NOT NULL,
    sign_count BIGINT NOT NULL DEFAULT 0,
    transports TEXT[] NOT NULL DEFAULT '{}',
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS passkeys_user_id_idx ON auth.passkeys (user_id);

-- One pending registration per user: beginning another replaces it.
CREATE TABLE IF NOT EXISTS auth.passkey_challenges (
    user_id UUID PRIMARY KEY REFERENCES auth.users (id) ON DELETE CASCADE,
    session_data JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE auth.refresh_tokens
ADD COLUMN IF NOT EXISTS auth_time TIMESTAMPTZ;

UPDATE auth.refresh_tokens SET auth_time = created_at
WHERE auth_time IS NULL;

ALTER TABLE auth.refresh_tokens ALTER COLUMN auth_time SET NOT NULL;
ALTER TABLE auth.refresh_tokens ALTER COLUMN auth_time SET DEFAULT now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.refresh_tokens DROP COLUMN IF EXISTS auth_time;
DROP TABLE IF EXISTS auth.passkey_challenges;
DROP TABLE IF EXISTS auth.passkeys;
-- +goose StatementEnd
