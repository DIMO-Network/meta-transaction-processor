package storage

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Storage interface {
	// New stores a newly created and submitted transaction.
	New(tx *Transaction) error
	// List returns the list of unconfirmed transactions, in order of ascending nonce.
	List() ([]*Transaction, error)
	SetTxMined(id string, block *Block) error
	Remove(id string) error
}

type Block struct {
	Number *big.Int
	Hash   common.Hash
}

type Transaction struct {
	ID string

	To   common.Address
	Data []byte

	Nonce    uint64
	GasPrice *big.Int
	Hash     common.Hash

	CreationBlock *Block
	MinedBlock    *Block
}

type memStorage struct {
	sync.Mutex
	storage map[string]*Transaction
}

func (s *memStorage) New(tx *Transaction) error {
	s.Lock()
	s.storage[tx.ID] = tx
	s.Unlock()
	return nil
}

func (s *memStorage) List() ([]*Transaction, error) {
	s.Lock()
	vals := maps.Values(s.storage)
	slices.SortFunc(vals, func(a, b *Transaction) bool {
		return a.Nonce < b.Nonce
	})
	s.Unlock()
	return vals, nil
}

func (s *memStorage) SetTxMined(id string, block *Block) error {
	s.Lock()
	s.storage[id].MinedBlock = block
	s.Unlock()
	return nil
}

func (s *memStorage) Remove(id string) error {
	s.Lock()
	delete(s.storage, id)
	s.Unlock()
	return nil
}

func NewMemStorage() Storage {
	return &memStorage{storage: make(map[string]*Transaction)}
}
