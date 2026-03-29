-- +goose Up
-- +goose StatementBegin
CREATE INDEX subscription_fk_link ON "subscription" (link_id);
CREATE INDEX subscription_fk_chat ON "subscription" (chat_id);
CREATE INDEX sub_tag_fk_tag ON "subscription_tag" (tag_id);
CREATE INDEX sub_tag_fk_sub ON "subscription_tag" (subscription_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF NOT EXISTS subscription_fk_link;
DROP INDEX IF NOT EXISTS subscription_fk_chat;
DROP INDEX IF NOT EXISTS sub_tag_fk_tag;
DROP INDEX IF NOT EXISTS sub_tag_fk_sub;
-- +goose StatementEnd
