-- +goose Up
-- +goose StatementBegin
-- How often a user opened each app per day. It only feeds the default phone
-- tab bar pins (the web reports an open when the user switches app); never per
-- item or URL. A new table, so no backfill. Rows older than 90 days are
-- deleted whenever the same user records an open.
CREATE TABLE IF NOT EXISTS global.app_opens (
    user_id TEXT NOT NULL,
    day DATE NOT NULL,
    app TEXT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day, app)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.app_opens;
-- +goose StatementEnd
