-- name: DeleteTagBySubscription :many
DELETE FROM "subscription_tag"
    WHERE subscription_id = $1
    RETURNING *;
