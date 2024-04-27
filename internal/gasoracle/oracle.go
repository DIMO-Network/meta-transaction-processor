package gasoracle

import (
	"context"
	"math/big"
)

type Oracle interface {
	EstimateFees(ctx context.Context) (EIP1559Fees, error)
}

type EIP1559Fees struct {
	GasTipCap *big.Int
	GasFeeCap *big.Int
}
