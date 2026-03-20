-- name: UpdateLink :one
UPDATE "link" SET
last_update = $2
    WHERE id = $1
    RETURNING *;
