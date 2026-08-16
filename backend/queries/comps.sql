-- name: CreateComp :one
INSERT INTO comps (search_id, external_id, title, price, currency, condition, buying_option, item_url, thumbnail_url, seller_country, excluded)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListCompsBySearch :many
SELECT * FROM comps WHERE search_id = $1 ORDER BY price ASC;

-- name: DeleteCompsBySearch :exec
DELETE FROM comps WHERE search_id = $1;
