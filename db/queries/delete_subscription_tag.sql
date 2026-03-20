-- name: DeleteTagBySubscription :exec
DELETE FROM "subscription_tag"
    WHERE subscription_id = $1;
