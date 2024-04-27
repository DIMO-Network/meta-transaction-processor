package gasoracle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
)

type polygon struct {
	endpoint string
	client   *http.Client
}

// Note that all fees from Polygon are in gwei = 10 ^ 9 wei.
type polygonOracleRes struct {
	Fast struct {
		MaxPriorityFee *big.Float `json:"maxPriorityFee"` // Unclear whether these floats are a good idea.
		MaxFee         *big.Float `json:"maxFee"`
	} `json:"fast"`
}

func NewPolygon(addr string) Oracle {
	return polygon{endpoint: addr, client: &http.Client{}}
}

var gweiToWei = big.NewFloat(1_000_000_000)

func (p polygon) EstimateFees(ctx context.Context) (EIP1559Fees, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint, nil)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed to construct request: %w", err)
	}

	res, err := p.client.Do(req)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed making request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return EIP1559Fees{}, fmt.Errorf("status code %d from request", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var por polygonOracleRes
	err = json.Unmarshal(b, &por)
	if err != nil {
		return EIP1559Fees{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	// TODO(elffjs): Check if there's any unexpected rounding.
	gasTipCap, _ := new(big.Float).Mul(gweiToWei, por.Fast.MaxPriorityFee).Int(nil)
	gasFeeCap, _ := new(big.Float).Mul(gweiToWei, por.Fast.MaxFee).Int(nil)

	return EIP1559Fees{
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
	}, nil
}
