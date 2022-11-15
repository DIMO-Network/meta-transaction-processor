-- +goose Up
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

ALTER TABLE meta_transaction_requests
    ADD COLUMN boosted_block_number numeric(78),
    ADD COLUMN boosted_block_hash bytea
        CONSTRAINT meta_transaction_requests_boosted_block_hash_check CHECK (length(boosted_block_hash) = 32);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET search_path TO meta_transaction_processor;

ALTER TABLE meta_transaction_requests
    DROP COLUMN boosted_block_number,
    DROP COLUMN boosted_block_hash;
-- +goose StatementEnd
