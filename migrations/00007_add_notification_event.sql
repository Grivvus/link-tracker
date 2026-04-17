-- +goose Up
-- +goose StatementBegin
CREATE TABLE "notification_event" (
    id BIGSERIAL PRIMARY KEY,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    failed BOOLEAN,
    failing_reason TEXT,
    sent BOOLEAN,
    link_id BIGINT NOT NULL,
    FOREIGN KEY (link_id) REFERENCES "link" (id) ON DELETE CASCADE,
    formed_notification JSONB NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "notification_event"
-- +goose StatementEnd
