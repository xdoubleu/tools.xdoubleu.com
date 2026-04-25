-- +goose Up
-- +goose StatementBegin

-- Add token_hash column, populate it from the existing plaintext token,
-- then swap it in as the new primary key.
-- Existing calendar URLs will stop working after this migration; users
-- must regenerate their feed tokens.
ALTER TABLE icsproxy.feeds ADD COLUMN token_hash TEXT;
UPDATE icsproxy.feeds SET token_hash = encode(sha256(token::BYTEA), 'hex');
ALTER TABLE icsproxy.feeds ALTER COLUMN token_hash SET NOT NULL;
ALTER TABLE icsproxy.feeds DROP CONSTRAINT feeds_pkey;
ALTER TABLE icsproxy.feeds ADD PRIMARY KEY (token_hash);
ALTER TABLE icsproxy.feeds DROP COLUMN token;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds ADD COLUMN token TEXT;
UPDATE icsproxy.feeds SET token = token_hash;
ALTER TABLE icsproxy.feeds ALTER COLUMN token SET NOT NULL;
ALTER TABLE icsproxy.feeds DROP CONSTRAINT feeds_pkey;
ALTER TABLE icsproxy.feeds ADD PRIMARY KEY (token);
ALTER TABLE icsproxy.feeds DROP COLUMN token_hash;
-- +goose StatementEnd
