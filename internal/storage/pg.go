package storage

import (
	"context"

	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	"github.com/DIMO-Network/shared/db"
	"github.com/ericlagergren/decimal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/types"
)

type dbStorage struct {
	DBS func() *db.ReaderWriter
}

func NewPGStorage(dbs func() *db.ReaderWriter) Storage {
	return &dbStorage{DBS: dbs}
}

func (s *dbStorage) New(tx *Transaction) error {
	mtr := models.MetaTransactionRequest{
		ID:                   tx.ID,
		Nonce:                types.NewDecimal(new(decimal.Big).SetUint64(tx.Nonce)),
		GasPrice:             types.NewDecimal(new(decimal.Big).SetBigMantScale(tx.GasPrice, 0)),
		To:                   tx.To[:],
		Data:                 tx.Data,
		Hash:                 tx.Hash[:],
		SubmittedBlockNumber: types.NewDecimal(new(decimal.Big).SetBigMantScale(tx.SubmittedBlock.Number, 0)),
		SubmittedBlockHash:   tx.SubmittedBlock.Hash[:],
	}

	err := mtr.Insert(context.Background(), s.DBS().Writer, boil.Infer())
	return err
}

func (s *dbStorage) List() ([]*Transaction, error) {
	mtrs, err := models.MetaTransactionRequests(
		qm.OrderBy(models.MetaTransactionRequestColumns.Nonce+" ASC"),
	).All(context.Background(), s.DBS().Reader)
	if err != nil {
		return nil, err
	}

	out := make([]*Transaction, len(mtrs))
	for i, mtr := range mtrs {
		out[i] = modelToTx(mtr)
	}

	return out, nil
}

func modelToTx(mod *models.MetaTransactionRequest) *Transaction {
	nonce, _ := mod.Nonce.Uint64()

	var minedBlock *Block

	if !mod.MinedBlockNumber.IsZero() {
		minedBlock = &Block{
			Number: mod.MinedBlockNumber.Int(nil),
			Hash:   common.BytesToHash(mod.MinedBlockHash.Bytes),
		}
	}

	return &Transaction{
		ID: mod.ID,

		To:   common.BytesToAddress(mod.To),
		Data: mod.Data,

		Nonce:    nonce,
		GasPrice: mod.GasPrice.Int(nil),
		Hash:     common.BytesToHash(mod.Hash),

		SubmittedBlock: &Block{
			Number: mod.SubmittedBlockNumber.Int(nil),
			Hash:   common.BytesToHash(mod.SubmittedBlockHash),
		},

		MinedBlock: minedBlock,
	}
}

func (s *dbStorage) SetMined(id string, block *Block) error {
	mtr, err := models.FindMetaTransactionRequest(context.Background(), s.DBS().Writer, id)
	if err != nil {
		return err
	}

	mtr.MinedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(block.Number, 0))
	mtr.MinedBlockHash = null.BytesFrom(block.Hash[:])

	_, err = mtr.Update(context.Background(), s.DBS().Writer, boil.Infer())
	return err
}

func (s *dbStorage) Remove(id string) error {
	mtr, err := models.FindMetaTransactionRequest(context.Background(), s.DBS().Writer, id)
	if err != nil {
		return err
	}

	_, err = mtr.Delete(context.Background(), s.DBS().Writer)
	return err
}

func NewDBStorage() Storage {
	return &dbStorage{}
}
