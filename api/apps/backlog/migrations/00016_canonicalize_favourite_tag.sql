-- +goose Up
-- Canonicalize legacy favourite spellings from Goodreads imports.
-- The app uses 'favourite' exclusively; imports may have produced
-- 'favorites', 'favourite', or 'favourites' as visible user tags.
-- After replacement, de-duplicate in case a row already had 'favourite'.
UPDATE backlog.user_books
SET
    tags = ARRAY(SELECT DISTINCT
        UNNEST(
            ARRAY_REPLACE(
                ARRAY_REPLACE(
                    ARRAY_REPLACE(tags, 'favorites', 'favourite'),
                    'favorite', 'favourite'
                ),
                'favourites', 'favourite'
            )
        )
    ORDER BY 1)
WHERE tags && ARRAY['favorites', 'favorite', 'favourites'];

-- +goose Down
-- Not reversible: original spelling is lost after canonicalisation.
