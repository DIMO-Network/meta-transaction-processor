package main

import (
	"math/big"
	"testing"
)

func TestFormatUnits(t *testing.T) {
	bigWei, _ := new(big.Int).SetString("787031054535065338726", 10) // the actual sweep balance
	cases := []struct {
		in   *big.Int
		dec  int
		want string
	}{
		{big.NewInt(1000000000000000000), 18, "1"},
		{big.NewInt(1500000000000000000), 18, "1.5"},
		{big.NewInt(1), 18, "0.000000000000000001"},
		{big.NewInt(0), 18, "0"},
		{big.NewInt(30000000000), 9, "30"},     // 30 gwei rendered with 9 decimals
		{big.NewInt(1500000000), 9, "1.5"},     // 1.5 gwei
		{bigWei, 18, "787.031054535065338726"}, // matches Polygonscan
	}
	for _, c := range cases {
		if got := formatUnits(c.in, c.dec); got != c.want {
			t.Errorf("formatUnits(%s, %d) = %s, want %s", c.in, c.dec, got, c.want)
		}
	}
}

func TestKnownSignerWallets(t *testing.T) {
	// The retired wallet we intend to sweep must be recognized.
	retired := knownSignerWallets[1]
	if !isKnownWallet(retired) {
		t.Fatalf("retired wallet %s not recognized as a known signer", retired)
	}
}
