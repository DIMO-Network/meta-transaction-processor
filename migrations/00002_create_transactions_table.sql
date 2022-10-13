-- +goose Up
-- +goose StatementBegin
SET search_path TO meta_transaction_processor, public;

CREATE TABLE meta_transaction_requests(
    id char(27) NOT NULL
        CONSTRAINT meta_transaction_requests_id_pkey PRIMARY KEY,
    nonce numeric(20) NOT NULL,
    to bytea NOT NULL
        CONSTRAINT meta_transaction_requests_to_check CHECK (length(to) = 32),
    data bytea,
    hash bytea
        CONSTRAINT meta_transaction_requests_hash_key UNIQUE,
        CONSTRAINT meta_transaction_requests_hash_check CHECK (length(hash) = 32),
    submitted_block_number numeric(78) NOT NULL,
    mined_block_number numeric(78),
    created_at timestamptz NOT NULL DEFAULT current_timestamp,
    updated_at timestamptz NOT NULL DEFAULT current_timestamp
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET search_path TO meta_transaction_processor, public;

DROP TABLE meta_transaction_requests;
-- +goose StatementEnd
