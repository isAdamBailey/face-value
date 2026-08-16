-- name: CreateSearch :one
INSERT INTO searches (user_email, image_key, image_width, image_height)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSearch :one
SELECT * FROM searches WHERE id = $1;

-- name: ListSearchesForUser :many
SELECT * FROM searches
WHERE user_email = $1
  AND (
    created_at < sqlc.arg(cursor_created_at)
    OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT $2;

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
