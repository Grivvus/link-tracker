-- name: AddNotificationEvent :one
INSERT INTO public."notification_event" (updated_at, link_id, formed_notification)
    VALUES ($1, $2, $3)
RETURNING *;