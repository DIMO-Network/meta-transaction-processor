package main

// TEMPORARY ops tooling: sweep NATIVE POL (the Polygon PoS gas token) stranded on a
// DECOMMISSIONED KMS signer wallet back to the still-active signer wallet.
//
// Context: this service's signer set (KMS_KEY_IDS) was reduced from two keys to one.
// The retired key's wallet still holds a little native POL (leftover gas). The KMS key
// can only sign from inside the cluster, so this one-off subcommand reuses the existing
// KMS signer to move that balance to the active wallet.
//
// This is NOT the Ethereum-mainnet ERC-20 `recover-pol` recovery. Here POL is the native
// gas currency, so the sweep is a plain value transfer (empty calldata), not a token
// transfer(). It runs on Polygon PoS (chain 137) and refuses any other chain.
//
// SAFE BY DEFAULT: without --broadcast it only simulates and prints what it would do.
// The destination is PINNED to the funding Ledger (an allowlist of one): the command
// refuses any other --to, so whoever can exec the pod can only ever consolidate funds to
// that safe wallet, never drain them elsewhere.
//
// It is deliberately self-contained in this one file and wired into main.go with a single
// dispatch case, so it can be reverted by deleting both.
//
//   meta-transaction-processor recover-native-pol --key-id <retiredKey> --amount all
//   meta-transaction-processor recover-native-pol --key-id <retiredKey> --amount all --broadcast
//   meta-transaction-processor recover-native-pol --key-id <retiredKey> --amount 5 --broadcast

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
	// Polygon PoS mainnet. Hardcoded on purpose: this recovery only ever runs on chain 137,
	// where POL is the native gas token. Refusing any other chain prevents pointing this at
	// the wrong network (e.g. the L1 where POL is an ERC-20 and a value transfer would be
	// a meaningless ETH send).
	polygonChainID = int64(137)

	// Native POL has 18 decimals, like other EVM native currencies.
	nativeDecimals = 18

	// gasLimitNativeTransfer is the intrinsic gas of a plain value transfer with empty
	// calldata. There is nothing to estimate -- it is always exactly 21000 -- but we still
	// call EstimateGas below as a liveness/sanity check and fall back to this constant.
	gasLimitNativeTransfer = uint64(21000)

	// Pinned recovery destination: the funding Ledger that tops up the signer wallets'
	// gas -- the same wallet used for the Ethereum POL recovery. This is an allowlist of
	// one; the command rejects any other --to, so this tool can only consolidate POL back
	// to the safe wallet, never send it anywhere else.
	pinnedDest = "0xB4536AfA9EE668E05503395AE17d2aD15CCd9069"
)

func recoverNativePOL(ctx context.Context, logger zerolog.Logger, settings *config.Settings, argv []string) {
	fs := flag.NewFlagSet("recover-native-pol", flag.ExitOnError)
	keyID := fs.String("key-id", "", "KMS key id/alias of the DECOMMISSIONED signer wallet to sweep (required)")
	toArg := fs.String("to", pinnedDest, "destination (pinned to the funding Ledger; any other value is rejected)")
	amountArg := fs.String("amount", "", "POL amount to send, or \"all\" to sweep the full balance minus gas (required)")
	rpcURL := fs.String("rpc", "", "Polygon PoS RPC URL (optional; defaults to the service's own ETHEREUM_RPC_URL)")
	expectFrom := fs.String("expect-from", "", "if set, abort unless the KMS key derives to this address (guards against a wrong --key-id)")
	broadcast := fs.Bool("broadcast", false, "actually send (otherwise dry-run / simulate only)")
	skipConfirm := fs.Bool("yes", false, "skip the interactive confirmation prompt")
	_ = fs.Parse(argv)

	die := func(msg string) {
		logger.Fatal().Msg(msg)
	}

	if *keyID == "" {
		die("--key-id required (the decommissioned signer's KMS key)")
	}
	if *amountArg == "" {
		die("--amount required (a number of POL, or \"all\")")
	}

	// Enforce the destination allowlist of one.
	if !common.IsHexAddress(*toArg) {
		die(fmt.Sprintf("--to %q is not a valid address", *toArg))
	}
	toAddr := common.HexToAddress(*toArg)
	if toAddr != common.HexToAddress(pinnedDest) {
		die(fmt.Sprintf("destination %s is not allowlisted; this tool only recovers to the funding Ledger %s", toAddr, pinnedDest))
	}

	// The service's own RPC is already Polygon PoS, so default to it; allow an override for
	// flexibility. The chain-id guard below is the real safety net regardless of source.
	rpc := *rpcURL
	if rpc == "" {
		rpc = settings.EthereumRPCURL
	}
	if rpc == "" {
		die("no RPC: pass --rpc or set ETHEREUM_RPC_URL in settings")
	}

	client, err := ethclient.DialContext(ctx, rpc)
	if err != nil {
		die("failed to dial RPC: " + err.Error())
	}
	defer client.Close()

	// Guard: must be Polygon PoS mainnet.
	chainID, err := client.ChainID(ctx)
	if err != nil {
		die("failed to read chain id: " + err.Error())
	}
	if chainID.Int64() != polygonChainID {
		die(fmt.Sprintf("RPC chain %d != %d (Polygon PoS); refusing to run", chainID.Int64(), polygonChainID))
	}

	// Build the KMS signer for the decommissioned wallet.
	kmsc, err := makeKMSClient(ctx, settings)
	if err != nil {
		die("failed to build KMS client: " + err.Error())
	}
	send, err := sender.FromKMS(ctx, kmsc, *keyID)
	if err != nil {
		die("failed to construct KMS signer: " + err.Error())
	}
	fromAddr := send.Address()

	// Guard against a fat-fingered key id pointing at the wrong wallet.
	if *expectFrom != "" {
		if !common.IsHexAddress(*expectFrom) {
			die(fmt.Sprintf("--expect-from %q is not a valid address", *expectFrom))
		}
		if fromAddr != common.HexToAddress(*expectFrom) {
			die(fmt.Sprintf("KMS key %s controls %s, not the expected %s; aborting", *keyID, fromAddr, *expectFrom))
		}
	}

	// Never sweep a wallet into itself.
	if fromAddr == toAddr {
		die(fmt.Sprintf("source and destination are the same wallet (%s); nothing to do", fromAddr))
	}

	bal, err := client.BalanceAt(ctx, fromAddr, nil)
	if err != nil {
		die("failed to read POL balance: " + err.Error())
	}
	if bal.Sign() <= 0 {
		die(fmt.Sprintf("wallet %s holds no POL; nothing to recover", fromAddr))
	}

	// Fees (EIP-1559). maxFee mirrors the existing recovery tool: 2*baseFee + tip.
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		die("failed to read latest header: " + err.Error())
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		die("failed to suggest gas tip: " + err.Error())
	}
	maxFee := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)

	// Sanity-check the intrinsic gas. A plain value transfer is always 21000; if the node
	// disagrees, trust it (and surface any error rather than silently assuming).
	gasLimit := gasLimitNativeTransfer
	if est, gErr := client.EstimateGas(ctx, ethereum.CallMsg{From: fromAddr, To: &toAddr}); gErr != nil {
		die("gas estimation failed: " + gErr.Error())
	} else if est > gasLimit {
		gasLimit = est
	}
	maxGasCost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), maxFee)

	// Resolve amount. For "all" we sweep the balance minus the reserved gas cost: you cannot
	// send the whole balance because the transfer itself must be paid for out of it.
	var value *big.Int
	sweepAll := strings.EqualFold(*amountArg, "all")
	if sweepAll {
		value = new(big.Int).Sub(bal, maxGasCost)
		if value.Sign() <= 0 {
			die(fmt.Sprintf("balance %s POL does not cover the max gas cost %s POL; nothing to sweep",
				formatUnits(bal, nativeDecimals), formatUnits(maxGasCost, nativeDecimals)))
		}
	} else {
		value, err = parseUnits(*amountArg, nativeDecimals)
		if err != nil {
			die("bad --amount: " + err.Error())
		}
		if value.Sign() <= 0 {
			die("amount must be > 0")
		}
		need := new(big.Int).Add(value, maxGasCost)
		if need.Cmp(bal) > 0 {
			die(fmt.Sprintf("amount %s POL + max gas %s POL > balance %s POL",
				formatUnits(value, nativeDecimals), formatUnits(maxGasCost, nativeDecimals), formatUnits(bal, nativeDecimals)))
		}
	}

	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		die("failed to read nonce: " + err.Error())
	}

	mode := "DRY RUN (simulate only)"
	if *broadcast {
		mode = "*** BROADCAST ***"
	}
	fmt.Println("\n====== NATIVE POL RECOVERY (Polygon PoS) ======")
	fmt.Printf("Mode:        %s\n", mode)
	fmt.Printf("Chain:       %d (Polygon PoS)\n", chainID.Int64())
	fmt.Printf("From:        %s  (decommissioned signer)\n", fromAddr)
	fmt.Printf("KMS key:     %s\n", *keyID)
	fmt.Printf("To:          %s  (funding Ledger)\n", toAddr)
	allNote := ""
	if sweepAll {
		allNote = "  (FULL BALANCE minus gas)"
	}
	fmt.Printf("Amount:      %s POL%s\n", formatUnits(value, nativeDecimals), allNote)
	fmt.Printf("Balance:     %s POL\n", formatUnits(bal, nativeDecimals))
	fmt.Printf("Nonce:       %d\n", nonce)
	fmt.Printf("Gas limit:   %d\n", gasLimit)
	fmt.Printf("Max fee/gas: %s gwei\n", formatUnits(maxFee, 9))
	fmt.Printf("Max gas cost:%s POL\n", formatUnits(maxGasCost, nativeDecimals))
	fmt.Println("===============================================")

	if !*broadcast {
		fmt.Println("\nDry run complete. Re-run with --broadcast to send for real.")
		return
	}

	if !*skipConfirm {
		fmt.Printf("\nType \"send\" to broadcast %s POL from %s to %s: ", formatUnits(value, nativeDecimals), fromAddr, toAddr)
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
		To:        &toAddr,
		Value:     value,
		Data:      nil,
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
	fmt.Printf("Explorer: https://polygonscan.com/tx/%s\n", hash.Hex())
}

// parseUnits converts a decimal string (e.g. "1.5") into a base-unit integer with the
// given number of decimals.
func parseUnits(s string, decimals int) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
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
