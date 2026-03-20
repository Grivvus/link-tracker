-- name: Delete :exec
DELETE FROM "chat"
    WHERE id = $1;
