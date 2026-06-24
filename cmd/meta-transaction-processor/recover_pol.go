package main

// TEMPORARY ops tooling: recover POL (Polygon Ecosystem Token) that landed by
// mistake in our KMS-backed reward wallets on ETHEREUM MAINNET, sweeping it back
// to the wallet that funded their gas.
//
// This is a port of the standalone `savepol` recovery onto the processor's existing
// KMS signer, so it can run from inside the cluster (the only place the minting-key
// KMS endpoint admits). It is deliberately self-contained in this one file and wired
// into main.go with a single dispatch case, so it can be reverted by deleting both.
//
// SAFE BY DEFAULT: without --broadcast it only simulates and prints what it would do.
//
//   meta-transaction-processor recover-pol --key-id <id> --rpc <L1_RPC> --amount 1
//   meta-transaction-processor recover-pol --key-id <id> --rpc <L1_RPC> --amount 1 --broadcast
//   meta-transaction-processor recover-pol --key-id <id> --rpc <L1_RPC> --amount all --broadcast
//
// The Ethereum-mainnet RPC is a REQUIRED flag and is never read from settings.yaml:
// the running service stays Polygon-only; only this manually-invoked command touches L1.

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
)

const (
	// Ethereum mainnet. Hardcoded on purpose: this recovery only ever runs on chain 1.
	recoverChainID = int64(1)

	// POL (Polygon Ecosystem Token) on Ethereum mainnet.
	// Verified on-chain: symbol "POL", decimals 18.
	polToken    = "0x455e53CBB86018Ac2B8092FdCd39d8444aFFC3F6"
	polDecimals = 18

	// Pinned recovery destination: the address that funded both reward wallets with
	// their gas ETH. This is an allowlist of one -- the command refuses any other --to,
	// so "anyone who can exec the pod" can only ever recover funds to the safe wallet,
	// never drain them elsewhere.
	pinnedDest = "0xB4536AfA9EE668E05503395AE17d2aD15CCd9069"

	// ERC-20 selectors.
	selTransfer  = "a9059cbb" // transfer(address,uint256)
	selBalanceOf = "70a08231" // balanceOf(address)
)

// The KMS-backed reward wallets that received POL by mistake on Ethereum. The derived
// KMS address MUST be one of these or we abort -- guards against pointing --key-id at
// some unrelated key.
var knownRewardWallets = []string{
	"0xcce4eF41A67E28C3CF3dbc51a6CD3d004F53aCBd", // wallet A
	"0xE92a1B8078EBc01412ba6Ef878425f35B32D6bBb", // wallet B
}

func recoverPOL(ctx context.Context, logger zerolog.Logger, settings *config.Settings, argv []string) {
	fs := flag.NewFlagSet("recover-pol", flag.ExitOnError)
	keyID := fs.String("key-id", "", "KMS key id/alias controlling the reward wallet (required)")
	rpcURL := fs.String("rpc", "", "Ethereum MAINNET RPC URL (required; never taken from settings)")
	amountArg := fs.String("amount", "", "POL amount to send, or \"all\" for the full balance (required)")
	toArg := fs.String("to", pinnedDest, "destination (pinned to the gas-funder; any other value is rejected)")
	broadcast := fs.Bool("broadcast", false, "actually send (otherwise dry-run / simulate only)")
	skipConfirm := fs.Bool("yes", false, "skip the interactive confirmation prompt")
	_ = fs.Parse(argv)

	die := func(msg string) {
		logger.Fatal().Msg(msg)
	}

	if *keyID == "" {
		die("--key-id required")
	}
	if *rpcURL == "" {
		die("--rpc required (Ethereum mainnet RPC; this service's own RPC is Polygon)")
	}
	if *amountArg == "" {
		die("--amount required (a number of POL, or \"all\")")
	}

	// Enforce the destination allowlist of one.
	toAddr := common.HexToAddress(*toArg)
	if toAddr != common.HexToAddress(pinnedDest) {
		die(fmt.Sprintf("destination %s is not allowlisted; this tool only recovers to the gas-funder %s", toAddr, pinnedDest))
	}

	client, err := ethclient.DialContext(ctx, *rpcURL)
	if err != nil {
		die("failed to dial RPC: " + err.Error())
	}
	defer client.Close()

	// Guard: must be Ethereum mainnet.
	chainID, err := client.ChainID(ctx)
	if err != nil {
		die("failed to read chain id: " + err.Error())
	}
	if chainID.Int64() != recoverChainID {
		die(fmt.Sprintf("RPC chain %d != %d (Ethereum mainnet); refusing to run", chainID.Int64(), recoverChainID))
	}

	// Build the KMS signer and verify it controls a wallet we recognize.
	kmsc, err := makeKMSClient(ctx, settings)
	if err != nil {
		die("failed to build KMS client: " + err.Error())
	}
	send, err := sender.FromKMS(ctx, kmsc, *keyID)
	if err != nil {
		die("failed to construct KMS signer: " + err.Error())
	}
	fromAddr := send.Address()
	if !isKnownWallet(fromAddr) {
		die(fmt.Sprintf("KMS key %s controls %s, which is not a known reward wallet; aborting", *keyID, fromAddr))
	}

	token := common.HexToAddress(polToken)

	// Balances.
	polBal, err := erc20BalanceOf(ctx, client, token, fromAddr)
	if err != nil {
		die("failed to read POL balance: " + err.Error())
	}
	ethBal, err := client.BalanceAt(ctx, fromAddr, nil)
	if err != nil {
		die("failed to read ETH balance: " + err.Error())
	}

	// Resolve amount.
	var amountWei *big.Int
	if strings.EqualFold(*amountArg, "all") {
		amountWei = new(big.Int).Set(polBal)
	} else {
		amountWei, err = parseUnits(*amountArg, polDecimals)
		if err != nil {
			die("bad --amount: " + err.Error())
		}
	}
	if amountWei.Sign() <= 0 {
		die("amount must be > 0")
	}
	if amountWei.Cmp(polBal) > 0 {
		die(fmt.Sprintf("amount %s > balance %s POL", formatUnits(amountWei, polDecimals), formatUnits(polBal, polDecimals)))
	}

	data := append(hexSelector(selTransfer), append(leftPad32(toAddr.Bytes()), leftPad32(amountWei.Bytes())...)...)

	// Fees (EIP-1559) and gas.
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		die("failed to read latest header: " + err.Error())
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		die("failed to suggest gas tip: " + err.Error())
	}
	maxFee := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)

	callMsg := ethereum.CallMsg{From: fromAddr, To: &token, Data: data}
	gasLimit, err := client.EstimateGas(ctx, callMsg)
	if err != nil {
		die("gas estimation failed (transfer would likely revert): " + err.Error())
	}

	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		die("failed to read nonce: " + err.Error())
	}

	maxGasCost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), maxFee)

	mode := "DRY RUN (simulate only)"
	if *broadcast {
		mode = "*** BROADCAST ***"
	}
	fmt.Println("\n========== POL RECOVERY ==========")
	fmt.Printf("Mode:        %s\n", mode)
	fmt.Printf("Chain:       %d (Ethereum mainnet)\n", chainID.Int64())
	fmt.Printf("From:        %s\n", fromAddr)
	fmt.Printf("KMS key:     %s\n", *keyID)
	fmt.Printf("To:          %s\n", toAddr)
	allNote := ""
	if strings.EqualFold(*amountArg, "all") {
		allNote = "  (FULL BALANCE)"
	}
	fmt.Printf("Amount:      %s POL%s\n", formatUnits(amountWei, polDecimals), allNote)
	fmt.Printf("Token:       %s\n", token)
	fmt.Printf("Nonce:       %d\n", nonce)
	fmt.Printf("Gas limit:   %d\n", gasLimit)
	fmt.Printf("Max fee/gas: %s gwei\n", formatUnits(maxFee, 9))
	fmt.Printf("Max gas cost:%s ETH  (wallet has %s ETH)\n", formatUnits(maxGasCost, 18), formatUnits(ethBal, 18))
	fmt.Println("==================================")

	if maxGasCost.Cmp(ethBal) > 0 {
		die("insufficient ETH for gas at current fees")
	}

	// Always simulate first so we never broadcast a tx that would revert.
	if _, err := client.CallContract(ctx, callMsg, nil); err != nil {
		die("simulation reverted: " + err.Error())
	}
	fmt.Println("\nSimulation: OK (transfer would succeed)")

	if !*broadcast {
		fmt.Println("\nDry run complete. Re-run with --broadcast to send for real.")
		return
	}

	if !*skipConfirm {
		fmt.Printf("\nType \"send\" to broadcast %s POL from %s to %s: ", formatUnits(amountWei, polDecimals), fromAddr, toAddr)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "send" {
			die("aborted by user")
		}
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: maxFee,
		Gas:       gasLimit,
		To:        &token,
		Value:     big.NewInt(0),
		Data:      data,
	})

	signer := types.LatestSignerForChainID(chainID)
	sig, err := send.Sign(ctx, signer.Hash(tx))
	if err != nil {
		die("KMS signing failed: " + err.Error())
	}
	signedTx, err := tx.WithSignature(signer, sig)
	if err != nil {
		die("failed to attach signature: " + err.Error())
	}

	fmt.Println("\nSigning via KMS and broadcasting...")
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		die("broadcast failed: " + err.Error())
	}
	hash := signedTx.Hash()
	fmt.Printf("tx hash: %s\n", hash.Hex())
	fmt.Println("waiting for confirmation...")

	rcpt, err := waitMined(ctx, client, hash)
	if err != nil {
		die("error waiting for receipt: " + err.Error())
	}
	statusStr := "FAILED"
	if rcpt.Status == types.ReceiptStatusSuccessful {
		statusStr = "SUCCESS"
	}
	fmt.Printf("\nStatus:   %s\n", statusStr)
	fmt.Printf("Block:    %d\n", rcpt.BlockNumber.Uint64())
	fmt.Printf("Gas used: %d\n", rcpt.GasUsed)
	fmt.Printf("Explorer: https://etherscan.io/tx/%s\n", hash.Hex())
}

func isKnownWallet(addr common.Address) bool {
	for _, w := range knownRewardWallets {
		if addr == common.HexToAddress(w) {
			return true
		}
	}
	return false
}

func erc20BalanceOf(ctx context.Context, client *ethclient.Client, token, holder common.Address) (*big.Int, error) {
	data := append(hexSelector(selBalanceOf), leftPad32(holder.Bytes())...)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(out), nil
}

func waitMined(ctx context.Context, client *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	deadline := time.NewTimer(5 * time.Minute)
	defer deadline.Stop()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		rcpt, err := client.TransactionReceipt(ctx, hash)
		if err == nil {
			return rcpt, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("timed out waiting for receipt for %s", hash.Hex())
		case <-tick.C:
		}
	}
}

func hexSelector(sel string) []byte {
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		fmt.Sscanf(sel[i*2:i*2+2], "%02x", &b[i])
	}
	return b
}

func leftPad32(b []byte) []byte {
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// parseUnits converts a decimal string (e.g. "1.5") into a base-unit integer with the
// given number of decimals.
func parseUnits(s string, decimals int) (*big.Int, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	if neg {
		return nil, fmt.Errorf("negative amount")
	}
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	if intPart == "" {
		intPart = "0"
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > decimals {
		return nil, fmt.Errorf("too many decimal places (max %d)", decimals)
	}
	frac = frac + strings.Repeat("0", decimals-len(frac))
	combined := intPart + frac
	v, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("not a valid decimal number: %q", s)
	}
	return v, nil
}

// formatUnits renders a base-unit integer as a decimal string with the given decimals.
func formatUnits(v *big.Int, decimals int) string {
	s := v.String()
	if decimals == 0 {
		return s
	}
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	if len(s) <= decimals {
		s = strings.Repeat("0", decimals-len(s)+1) + s
	}
	intPart := s[:len(s)-decimals]
	fracPart := strings.TrimRight(s[len(s)-decimals:], "0")
	out := intPart
	if fracPart != "" {
		out = intPart + "." + fracPart
	}
	if neg {
		out = "-" + out
	}
	return out
}
