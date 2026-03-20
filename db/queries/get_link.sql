-- name: GetLinkByID :one
SELECT id, url, last_update from "link"
    WHERE id = $1;
