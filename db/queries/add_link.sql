-- name: AddLink :one
INSERT INTO "link" (url)
    VALUES ($1)
RETURNING *;
