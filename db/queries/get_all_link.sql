-- name: GetAllLinksPaginate :many
SELECT id, url, last_update from "link"
    WHERE id > $1
    ORDER BY id ASC
    LIMIT $2;
