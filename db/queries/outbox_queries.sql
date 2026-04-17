-- name: GetPendingWithLimit :many
SELECT * FROM public."notification_event"
    WHERE sent IS NOT true AND failed IS NOT true
    AND id > $1
    ORDER BY id ASC
LIMIT $2;

-- name: MarkSent :exec
UPDATE public."notification_event" SET
    sent = true
WHERE id = $1;

-- name: MarkFailed :exec
UPDATE public."notification_event" SET
    failed = true,
    failing_reason = $2
WHERE id = $1;