-- name: RegisterChat :exec
INSERT INTO "chat" (id)
    VALUES ($1);
