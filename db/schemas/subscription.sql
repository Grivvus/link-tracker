CREATE TABLE "subscription" (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT,
    chat_id BIGINT,
    FOREIGN KEY (link_id) REFERENCES "link" (id) ON DELETE RESTRICT,
    FOREIGN KEY (chat_id) REFERENCES "chat" (id) ON DELETE CASCADE
);
