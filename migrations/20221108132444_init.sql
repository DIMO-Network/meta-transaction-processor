-- +goose Up
-- +goose StatementBegin
REVOKE CREATE ON schema public FROM public;
CREATE SCHEMA IF NOT EXISTS meta_transaction_processor;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA meta_transaction_processor CASCADE;
GRANT CREATE, USAGE ON schema public TO public;
-- +goose StatementEnd
