-- +goose Up
-- +goose StatementBegin
CREATE TABLE "tag" (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "tag";
-- +goose StatementEnd
