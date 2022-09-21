package announcer

import (
	"github.com/DIMO-Network/meta-transaction-processor/storage"
	"github.com/ethereum/go-ethereum/common"
)

type CreationAnnouncement struct {
	ID    string
	Hash  common.Hash
	Block *storage.Block
}

type MinedAnnouncement struct {
	ID         string
	Hash       common.Hash
	Block      *storage.Block
	Successful bool
}

type ConfirmedAnnouncement struct {
	ID         string
	Hash       common.Hash
	Block      *storage.Block
	Logs       []*Log
	Successful bool
}

type Log struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

type Announcer interface {
	Created(ann *CreationAnnouncement)
}
