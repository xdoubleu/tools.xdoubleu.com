-- +goose Up
-- +goose StatementBegin
-- A paused chat has stopped its VM and freed its runner slot; its worktree and
-- transcript stay. last_message is the person's newest message, shown in the
-- chat list. events_purged_at marks a closed chat whose events were deleted;
-- the session row stays as its summary.
ALTER TABLE factory.chat_sessions
DROP CONSTRAINT IF EXISTS chat_sessions_state_check;
ALTER TABLE factory.chat_sessions
ADD CONSTRAINT chat_sessions_state_check CHECK (
    state IN ('idle', 'working', 'asking', 'paused', 'stopped')
);

ALTER TABLE factory.chat_sessions
ADD COLUMN IF NOT EXISTS last_message TEXT NOT NULL DEFAULT '';
ALTER TABLE factory.chat_sessions
ADD COLUMN IF NOT EXISTS events_purged_at TIMESTAMPTZ;

-- Backfill from the transcript, so existing chats list their last message.
UPDATE factory.chat_sessions AS s
SET last_message = left(e.payload, 200)
FROM (
    SELECT DISTINCT ON (session_id)
        session_id,
        payload
    FROM factory.chat_events
    WHERE kind IN ('user_message', 'user_answer')
    ORDER BY session_id ASC, seq DESC
) AS e
WHERE s.id = e.session_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE factory.chat_sessions SET state = 'idle'
WHERE state = 'paused';
ALTER TABLE factory.chat_sessions DROP COLUMN IF EXISTS events_purged_at;
ALTER TABLE factory.chat_sessions DROP COLUMN IF EXISTS last_message;
ALTER TABLE factory.chat_sessions
DROP CONSTRAINT IF EXISTS chat_sessions_state_check;
ALTER TABLE factory.chat_sessions
ADD CONSTRAINT chat_sessions_state_check CHECK (
    state IN ('idle', 'working', 'asking', 'stopped')
);
-- +goose StatementEnd
