-- name: GetChat :one
SELECT id FROM "chat"
    WHERE id = $1;
