-- name: CreateSearch :one
INSERT INTO searches (user_email, image_key, image_width, image_height)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSearch :one
SELECT * FROM searches WHERE id = $1;

-- name: ListSearches :many
-- Every signed-in user sees every search, so this is deliberately not
-- scoped to the caller. The row-constructor comparison is what lets the
-- keyset seek straight to the cursor on searches_created_idx instead of
-- rescanning from the newest row on every page.
SELECT * FROM searches
WHERE (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListSearchesByUser :many
-- The filtered variant is a separate statement rather than an
-- `email IS NULL OR ...` predicate: a prepared generic plan can't fold that
-- away, so the email would degrade from an index qual to a row filter.
SELECT * FROM searches
WHERE user_email = sqlc.arg(user_email)
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListSearchUsers :many
SELECT DISTINCT user_email FROM searches ORDER BY user_email;

-- name: SetSearchIdentification :one
UPDATE searches
SET status = 'pricing',
    title = $2,
    brand = $3,
    model = $4,
    category = $5,
    condition_notes = $6,
    search_query = $7,
    vision_model = $8,
    vision_raw = $9,
    confidence = $10
WHERE id = $1
RETURNING *;

-- name: SetSearchComplete :one
UPDATE searches
SET status = 'complete',
    price_source = $2,
    currency = $3,
    comp_count = $4,
    price_mean = $5,
    price_median = $6,
    price_min = $7,
    price_max = $8,
    price_trimmed_mean = $9,
    completed_at = now()
WHERE id = $1
RETURNING *;

-- name: SetSearchFailed :one
UPDATE searches
SET status = 'failed', error_message = $2
WHERE id = $1
RETURNING *;

-- name: UpdateSearchQuery :one
UPDATE searches
SET status = 'pricing', search_query = $2
WHERE id = $1
RETURNING *;

-- name: DeleteSearch :exec
DELETE FROM searches WHERE id = $1;

-- name: MarkStaleSearchesFailed :exec
UPDATE searches
SET status = 'failed', error_message = 'orphaned by server restart'
WHERE status IN ('pending', 'identifying', 'pricing')
  AND created_at < now() - interval '5 minutes';
