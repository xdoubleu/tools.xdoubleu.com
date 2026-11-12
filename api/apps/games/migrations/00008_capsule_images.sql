-- Game artwork moves from Steam's 32px icons and 460px headers to the 616px
-- store capsule, which is derived from the app id alone.

-- +goose Up
-- +goose StatementBegin
UPDATE games.steam_games
SET
    image_url = 'https://shared.cloudflare.steamstatic.com/'
    || 'store_item_assets/steam/apps/' || id || '/capsule_616x353.jpg'
WHERE
    image_url LIKE '%steampowered.com/%'
    OR image_url LIKE '%steamstatic.com/%';

UPDATE games.steam_wishlist
SET
    image_url = 'https://shared.cloudflare.steamstatic.com/'
    || 'store_item_assets/steam/apps/' || game_id || '/capsule_616x353.jpg'
WHERE
    image_url LIKE '%steampowered.com/%'
    OR image_url LIKE '%steamstatic.com/%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op: the old icon and header URLs are not recoverable; a re-sync rewrites
-- them.
SELECT 1;
-- +goose StatementEnd
