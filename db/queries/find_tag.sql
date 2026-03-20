-- name: FindTag :one
SELECT id, name from "tag"
    WHERE name = $1;
