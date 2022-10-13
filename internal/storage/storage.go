package storage

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/maps"
)

type Storage interface {
	New(tx *Transaction) error
	List() ([]*Transaction, error)
	SetTxMined(id string, block *big.Int) error
	Remove(id string) error
}

type Block struct {
	Number    *big.Int
	Hash      common.Hash
	Timestamp uint64
}

type Transaction struct {
	ID string

	To   common.Address
	Data []byte

	Nonce uint64
	Hash  common.Hash

	CreationBlockNumber *big.Int
	MinedBlockNumber    *big.Int
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
	return maps.Values(s.storage), nil
}

func (s *memStorage) SetTxMined(id string, blockNumber *big.Int) error {
	s.Lock()
	s.storage[id].MinedBlockNumber = blockNumber
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
