-- +goose Up
-- +goose StatementBegin
CREATE TABLE "subscription" (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    FOREIGN KEY (link_id) REFERENCES "link" (id) ON DELETE RESTRICT,
    FOREIGN KEY (chat_id) REFERENCES "chat" (id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "subscription";
-- +goose StatementEnd
