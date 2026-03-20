-- name: FindSubscriptionsByChat :many
SELECT id, link_id, chat_id from "subscription"
    WHERE chat_id = $1;
