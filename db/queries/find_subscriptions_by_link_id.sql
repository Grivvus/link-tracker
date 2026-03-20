-- name: FindSubscriptionsByLink :many
SELECT id, link_id, chat_id from "subscription"
    WHERE link_id = $1;
