package config

type Settings struct {
	// EthereumRPCURL is the URL of the JSON-RPC endpoint to use for
	// blockchain interactions.
	EthereumRPCURL string `yaml:"ETHEREUM_RPC_URL"`

	// EthereumChainID is the chain ID of the target blockchain. One standard
	// list of these is https://chainlist.org.
	EthereumChainID int `yaml:"ETHEREUM_CHAIN_ID"`

	// SenderPrivateKey is a hex-encoded private key for the secp256k1 curve, used
	// to sign transactions. This should only be used for testing.
	SenderPrivateKey string `yaml:"SENDER_PRIVATE_KEY"`

	// ConfirmationBlocks is the number of blocks needed to consider a
	// transaction confirmed.
	ConfirmationBlocks int `yaml:"CONFIRMATION_BLOCKS"`

	// KafkaServers is a comma-seperated list of Kafka bootstrap servers.
	// We typically only specify one.
	KafkaServers string `json:"KAFKA_SERVERS"`
}
