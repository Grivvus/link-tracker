-- name: RegisterChat :exec
INSERT INTO "chat" (id)
    VALUES ($1);

-- name: GetChat :one
SELECT id FROM "chat"
    WHERE id = $1;

-- name: DeleteChat :one
DELETE FROM "chat"
    WHERE id = $1
RETURNING *;
