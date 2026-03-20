-- name: Unsubscribe :one
DELETE FROM "subscription"
    WHERE id = $1
    RETURNING *;
