-- +goose Up
-- +goose StatementBegin
CREATE TABLE "tracking" (
    link_id INTEGER,
    chat_id INTEGER,
    PRIMARY KEY (link_id, chat_id),
    FOREIGN KEY (link_id) REFERENCES "link" (id) ON DELETE CASCADE,
    FOREIGN KEY (chat_id) REFERENCES "chat" (id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "tracking";
-- +goose StatementEnd
