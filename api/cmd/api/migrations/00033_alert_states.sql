-- +goose Up
-- +goose StatementBegin
-- Breach/recovery state for jobs.ThresholdAlertJob (issue #1283). Unlike
-- global.notified_issues (append-only, never cleared), a rule here re-arms
-- on recovery: breaching flips back to false and since is cleared, so a
-- second incident is notified again instead of staying silently deduped
-- forever. current_value/threshold are refreshed on every evaluation
-- (not just on a breach/recovery transition) so get_alert_states always
-- reflects the latest reading, not just the last time an email went out.
CREATE TABLE global.alert_states (
    rule_key TEXT PRIMARY KEY,
    breaching BOOLEAN NOT NULL DEFAULT FALSE,
    since TIMESTAMPTZ,
    last_notified_at TIMESTAMPTZ,
    current_value DOUBLE PRECISION,
    threshold DOUBLE PRECISION
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.alert_states;
-- +goose StatementEnd
