-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.user_books
DROP CONSTRAINT IF EXISTS chk_user_books_rating;
ALTER TABLE backlog.user_books
ADD CONSTRAINT chk_user_books_rating CHECK (
    rating IS NULL OR rating BETWEEN 1 AND 5
),
ALTER COLUMN tags SET NOT NULL,
ALTER COLUMN tags SET DEFAULT '{}';

CREATE OR REPLACE FUNCTION backlog.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_user_books_updated_at ON backlog.user_books;
CREATE TRIGGER trg_user_books_updated_at
BEFORE UPDATE ON backlog.user_books
FOR EACH ROW EXECUTE FUNCTION backlog.set_updated_at();

DROP TRIGGER IF EXISTS trg_user_integrations_updated_at
ON backlog.user_integrations;
CREATE TRIGGER trg_user_integrations_updated_at
BEFORE UPDATE ON backlog.user_integrations
FOR EACH ROW EXECUTE FUNCTION backlog.set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_user_integrations_updated_at
ON backlog.user_integrations;
DROP TRIGGER IF EXISTS trg_user_books_updated_at ON backlog.user_books;
DROP FUNCTION IF EXISTS backlog.set_updated_at;
ALTER TABLE backlog.user_books
DROP CONSTRAINT IF EXISTS chk_user_books_rating;
-- +goose StatementEnd
