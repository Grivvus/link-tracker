-- name: FindLinkByURL :one
SELECT id, url, last_update from "link"
    WHERE url = $1;
