-- +goose Up
-- +goose StatementBegin
-- A book on the read shelf is fully read: status 'read' forces 100%
-- progress (pages mode moves to the last page, or to percent mode when the
-- page count is unknown). Leaving 'read' starts a new read at 0;
-- finished_at keeps the history of earlier reads.
CREATE OR REPLACE FUNCTION books.sync_read_status_progress()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    pages INTEGER;
BEGIN
    IF NEW.status = 'read' THEN
        NEW.progress_percent = 100;
        IF NEW.progress_mode = 'pages' THEN
            SELECT page_count INTO pages FROM books.books WHERE id = NEW.book_id;
            IF pages IS NOT NULL AND pages > 0 THEN
                NEW.current_page = pages;
            ELSE
                NEW.progress_mode = 'percent';
            END IF;
        END IF;
    ELSIF TG_OP = 'UPDATE' AND OLD.status = 'read' THEN
        NEW.progress_percent = 0;
        NEW.current_page = 0;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_user_books_read_progress
BEFORE INSERT OR UPDATE ON books.user_books
FOR EACH ROW EXECUTE FUNCTION books.sync_read_status_progress();

-- Backfill: the trigger rewrites every existing read row's progress.
UPDATE books.user_books SET status = status
WHERE status = 'read';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER trg_user_books_read_progress ON books.user_books;
DROP FUNCTION books.sync_read_status_progress();
-- +goose StatementEnd
