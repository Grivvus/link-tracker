-- name: AddSubscriptionTag :exec
INSERT INTO "subscription_tag" (subscription_id, tag_id)
    VALUES ($1, $2);
