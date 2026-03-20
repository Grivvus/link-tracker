-- name: FindTagBySubscription :many
SELECT "tag".id as tag_id, "tag".name as name from 
    "tag" inner join "subscription_tag" 
    on "tag".id = "subscription_tag".tag_id
        WHERE "subscription_tag".subscription_id = $1;
