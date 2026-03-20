-- +goose Up
-- +goose StatementBegin
CREATE TABLE "subscription_tag" (
    subscription_id BIGINT,
    tag_id BIGINT,
    PRIMARY KEY (subscription_id, tag_id),
    FOREIGN KEY (subscription_id) REFERENCES "subscription" (id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES "tag" (id) ON DELETE RESTRICT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "subscription_tag";
-- +goose StatementEnd
