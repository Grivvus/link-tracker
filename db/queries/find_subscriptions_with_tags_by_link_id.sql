-- name: FindSubscriptionsWithTagsByLink :many
SELECT s.id, s.link_id, s.chat_id, t.name
FROM "subscription" as s
LEFT JOIN "subscription_tag" as st ON (st.subscription_id = s.id)
LEFT JOIN "tag" as t ON (t.id = st.tag_id)
    WHERE link_id = $1;
