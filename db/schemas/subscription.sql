CREATE TABLE "subscription" (
    id SERIAL PRIMARY KEY,
    link_id INTEGER,
    chat_id INTEGER,
    FOREIGN KEY (link_id) REFERENCES "link" (id) ON DELETE RESTRICT,
    FOREIGN KEY (chat_id) REFERENCES "chat" (id) ON DELETE CASCADE
);
