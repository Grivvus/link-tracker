-- name: Subscribe :one
INSERT INTO "subscription" (link_id, chat_id)
    VALUES ($1, $2)
    RETURNING *;
