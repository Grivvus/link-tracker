-- name: FindSubscriptionByLinkAndChat :one
SELECT * FROM "subscription"
    WHERE chat_id = $1 AND link_id = $2;
