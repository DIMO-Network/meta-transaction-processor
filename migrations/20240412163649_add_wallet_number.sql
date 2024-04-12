-- +goose Up
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

ALTER TABLE meta_transaction_requests ADD COLUMN wallet_index integer;
UPDATE meta_transaction_requests SET wallet_index = 0;
ALTER TABLE meta_transaction_requests ALTER COLUMN wallet_index SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

ALTER TABLE meta_transaction_requests DROP COLUMN wallet_index;
-- +goose StatementEnd
