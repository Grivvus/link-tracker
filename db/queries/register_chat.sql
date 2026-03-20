-- name: Register :exec
INSERT INTO "chat" (id)
    VALUES ($1);
