-- name: AddTag :one
INSERT INTO "tag" (name)
    VALUES ($1)
    RETURNING *;
