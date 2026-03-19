-- +goose Up
-- +goose StatementBegin
CREATE TABLE "link" (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    last_update TIMESTAMP WITH TIME ZONE DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "link";
-- +goose StatementEnd
