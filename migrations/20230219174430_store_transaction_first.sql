-- +goose Up
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

ALTER TABLE meta_transaction_requests
    ALTER COLUMN submitted_block_number DROP NOT NULL,
    ALTER COLUMN submitted_block_hash DROP NOT NULL,
    ALTER COLUMN nonce DROP NOT NULL,
    ALTER COLUMN gas_price DROP NOT NULL,
    ALTER COLUMN hash DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

-- High chance of failure.
ALTER TABLE meta_transaction_requests
    ALTER COLUMN submitted_block_number SET NOT NULL,
    ALTER COLUMN submitted_block_hash SET NOT NULL,
    ALTER COLUMN nonce SET NOT NULL,
    ALTER COLUMN hash SET NOT NULL,
    ALTER COLUMN gas_price SET NOT NULL;
-- +goose StatementEnd
