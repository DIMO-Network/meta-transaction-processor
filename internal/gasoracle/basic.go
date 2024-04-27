package gasoracle

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type basic struct {
	client *ethclient.Client
}

func NewBasic(client *ethclient.Client) Oracle {
	return basic{client: client}
}

func (b basic) EstimateFees(ctx context.Context) (EIP1559Fees, error) {
	h, err := b.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed to retrieve latest block header: %w", err)
	}

	gasTipCap, err := b.client.SuggestGasTipCap(ctx)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed to retrieve gas tip cap: %w", err)
	}

	gasFeeCap := new(big.Int).Add(
		new(big.Int).Mul(common.Big2, h.BaseFee),
		gasTipCap,
	)

	return EIP1559Fees{
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
	}, nil
}
