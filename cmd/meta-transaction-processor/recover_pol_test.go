package main

import (
	"math/big"
	"testing"
)

func TestParseUnits(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1", "1000000000000000000"},
		{"1.5", "1500000000000000000"},
		{"0.000000000000000001", "1"},
		{"1234.567", "1234567000000000000000"},
		{"0", "0"},
		{".5", "500000000000000000"},
		{"  2  ", "2000000000000000000"},
	}
	for _, c := range cases {
		got, err := parseUnits(c.in, 18)
		if err != nil {
			t.Fatalf("parseUnits(%q) unexpected error: %v", c.in, err)
		}
		if got.String() != c.want {
			t.Errorf("parseUnits(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestParseUnitsErrors(t *testing.T) {
	for _, in := range []string{
		"1.0000000000000000001", // 19 fractional digits > 18 decimals
		"-1",
		"abc",
		"1.2.3",
	} {
		if _, err := parseUnits(in, 18); err == nil {
			t.Errorf("parseUnits(%q) expected error, got nil", in)
		}
	}
}

func TestOnePOLIs1e18(t *testing.T) {
	one, err := parseUnits("1", polDecimals)
	if err != nil {
		t.Fatal(err)
	}
	exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	if one.Cmp(exp) != 0 {
		t.Fatalf("1 POL = %s, want %s", one, exp)
	}
}

func TestFormatUnits(t *testing.T) {
	cases := []struct {
		in   *big.Int
		dec  int
		want string
	}{
		{big.NewInt(1000000000000000000), 18, "1"},
		{big.NewInt(1500000000000000000), 18, "1.5"},
		{big.NewInt(1), 18, "0.000000000000000001"},
		{big.NewInt(343568396), 9, "0.343568396"},
		{big.NewInt(0), 18, "0"},
	}
	for _, c := range cases {
		if got := formatUnits(c.in, c.dec); got != c.want {
			t.Errorf("formatUnits(%s, %d) = %s, want %s", c.in, c.dec, got, c.want)
		}
	}
}

// parseUnits and formatUnits must round-trip for any value parseUnits accepts.
func TestParseFormatRoundTrip(t *testing.T) {
	for _, in := range []string{"1", "1.5", "0.000000000000000001", "1234.567", "0"} {
		v, err := parseUnits(in, 18)
		if err != nil {
			t.Fatal(err)
		}
		back, err := parseUnits(formatUnits(v, 18), 18)
		if err != nil {
			t.Fatalf("re-parsing formatUnits(%q) failed: %v", in, err)
		}
		if back.Cmp(v) != 0 {
			t.Errorf("round-trip %q: %s != %s", in, back, v)
		}
	}
}
