package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

// sweep-native is a one-off operational subcommand for emptying the native POL
// balance of a decommissioned signer wallet back to the gas funder. It is
// deliberately narrow:
//
//   - Polygon only. It uses the service's own configured RPC (EthereumRPCURL)
//     and refuses to run unless the chain id is 137. There is no flag to point
//     it at another chain.
//   - The destination is pinned to the gas funder. --to defaults to it and any
//     other value is rejected, so a fat-fingered address can't send funds astray.
//   - The signing key must resolve to one of the known signer wallets, so a
//     wrong --key-id aborts before anything is built.
//   - It does nothing without --broadcast; the default is a dry run that prints
//     exactly what it would send.
//
// This whole file is meant to be reverted once the sweep is done.

const (
	// polygonChainID is the only chain this command will operate on.
	polygonChainID = int64(137)

	// sweepDest is the gas funder. The retired wallet's leftover gas goes here
	// and nowhere else.
	sweepDest = "0xB4536AfA9EE668E05503395AE17d2aD15CCd9069"

	// nativeTransferGas is the intrinsic gas for a plain value transfer to an
	// account. We confirm it against EstimateGas before using it.
	nativeTransferGas = uint64(21000)
)

func sweepNative(ctx context.Context, logger zerolog.Logger, settings *config.Settings, argv []string) {
	fs := flag.NewFlagSet("sweep-native", flag.ExitOnError)
	keyID := fs.String("key-id", "", "AWS KMS key id (or alias) of the wallet to sweep (required)")
	to := fs.String("to", sweepDest, "destination address (must be the pinned gas funder)")
	broadcast := fs.Bool("broadcast", false, "actually send the transaction; default is a dry run")
	yes := fs.Bool("yes", false, "skip the interactive confirmation prompt")
	_ = fs.Parse(argv)

	if *keyID == "" {
		logger.Fatal().Msg("--key-id is required.")
	}

	// Destination allowlist of one.
	if !strings.EqualFold(*to, sweepDest) {
		logger.Fatal().Msgf("Refusing to sweep to %s; the only permitted destination is the gas funder %s.", *to, sweepDest)
	}
	toAddr := common.HexToAddress(sweepDest)

	// The service's RPC is Polygon. We use it directly rather than taking an
	// RPC flag, and we verify the chain below.
	client, err := ethclient.DialContext(ctx, settings.EthereumRPCURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to dial the Ethereum RPC.")
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to retrieve chain id.")
	}
	if chainID.Int64() != polygonChainID {
		logger.Fatal().Msgf("Connected to chain %d, but this command only runs on Polygon (%d).", chainID, polygonChainID)
	}

	// Build the KMS-backed signer and confirm it's one of our wallets.
	kmsc, err := makeKMSClient(ctx, settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load AWS config.")
	}
	send, err := sender.FromKMS(ctx, kmsc, *keyID)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to construct the signer from KMS.")
	}
	from := send.Address()
	if !isKnownWallet(from) {
		logger.Fatal().Msgf("Key %s resolves to %s, which is not a known signer wallet; aborting.", *keyID, from)
	}

	logger.Info().Msgf("Sweeping native POL from %s to %s on Polygon.", from, toAddr)

	// Current balance.
	balance, err := client.BalanceAt(ctx, from, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to read balance.")
	}
	if balance.Sign() == 0 {
		logger.Info().Msg("Balance is zero; nothing to sweep.")
		return
	}

	// EIP-1559 fees. Cap the base-fee component at 2x the current base fee so
	// the transaction stays valid through a couple of busy blocks.
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to read latest header.")
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to suggest a gas tip.")
	}
	maxFee := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)

	// Confirm the gas limit against the node. A transfer to an EOA is 21000;
	// EstimateGas will tell us if the funder is unexpectedly a contract. We
	// estimate with a zero value so the call can't fail an affordability check.
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &toAddr, Value: big.NewInt(0)})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to estimate gas.")
	}
	if gasLimit < nativeTransferGas {
		gasLimit = nativeTransferGas
	}

	// Reserve the worst-case fee and send everything else.
	maxGasCost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), maxFee)
	value := new(big.Int).Sub(balance, maxGasCost)
	if value.Sign() <= 0 {
		logger.Fatal().Msgf("Balance %s POL does not cover the reserved gas cost %s POL; nothing to sweep.",
			formatUnits(balance, 18), formatUnits(maxGasCost, 18))
	}

	logger.Info().Msgf("Balance:      %s POL", formatUnits(balance, 18))
	logger.Info().Msgf("Max gas cost: %s POL (gas %d @ maxFee %s gwei)", formatUnits(maxGasCost, 18), gasLimit, formatUnits(maxFee, 9))
	logger.Info().Msgf("Will send:    %s POL to %s", formatUnits(value, 18), toAddr)

	if !*broadcast {
		logger.Info().Msg("Dry run (no --broadcast); not sending. Re-run with --broadcast to execute.")
		return
	}

	if !*yes {
		fmt.Printf("\nType \"send\" to broadcast the transfer of %s POL to %s: ", formatUnits(value, 18), toAddr)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "send" {
			logger.Info().Msg("Confirmation not given; aborting.")
			return
		}
	}

	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to read nonce.")
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &toAddr,
		Value:     value,
		Gas:       gasLimit,
		GasFeeCap: maxFee,
		GasTipCap: tip,
	})

	signer := types.LatestSignerForChainID(chainID)
	sig, err := send.Sign(ctx, signer.Hash(tx))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to sign the transaction.")
	}
	signedTx, err := tx.WithSignature(signer, sig)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to attach the signature.")
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to broadcast the transaction.")
	}
	logger.Info().Msgf("Broadcast transaction %s; waiting for it to be mined.", signedTx.Hash())

	rec, err := waitMined(ctx, client, signedTx.Hash())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed while waiting for the receipt.")
	}
	if rec.Status != types.ReceiptStatusSuccessful {
		logger.Fatal().Msgf("Transaction %s reverted (status 0).", signedTx.Hash())
	}
	logger.Info().Msgf("Done. %s POL swept to %s in block %d.", formatUnits(value, 18), toAddr, rec.BlockNumber)
}

// isKnownWallet reports whether addr is one of the known signer wallets.
func isKnownWallet(addr common.Address) bool {
	for _, w := range knownSignerWallets {
		if addr == w {
			return true
		}
	}
	return false
}

// knownSignerWallets are the two reward signer addresses. The sweep key must
// resolve to one of these.
var knownSignerWallets = []common.Address{
	common.HexToAddress("0xcce4eF41A67E28C3CF3dbc51a6CD3d004F53aCBd"),
	common.HexToAddress("0xE92a1B8078EBc01412ba6Ef878425f35B32D6bBb"),
}

// waitMined polls for the receipt of txHash, giving up after five minutes.
func waitMined(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		rec, err := client.TransactionReceipt(ctx, txHash)
		if err == nil {
			return rec, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for receipt: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// formatUnits renders a wei-scale integer as a decimal string with the given
// number of fractional digits, trimming trailing zeros.
func formatUnits(v *big.Int, decimals int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole := new(big.Int).Quo(v, scale)
	frac := new(big.Int).Rem(new(big.Int).Abs(v), scale)
	if frac.Sign() == 0 {
		return whole.String()
	}
	fracStr := fmt.Sprintf("%0*s", decimals, frac.String())
	fracStr = strings.TrimRight(fracStr, "0")
	return whole.String() + "." + fracStr
}
