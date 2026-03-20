-- name: DeleteChat :one
DELETE FROM "chat"
    WHERE id = $1
    RETURNING *;
