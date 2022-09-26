package config

type Settings struct {
	// MonitoringPort is the port on which we run the health check endpoint and
	// serve Prometheus metrics.
	MonitoringPort string `json:"MONITORING_PORT"`

	// KafkaServers is a comma-seperated list of Kafka bootstrap servers.
	// We typically only specify one.
	KafkaServers string `json:"KAFKA_SERVERS"`

	// ConsumerGroupName is the name of the consumer group.

	// TransactionRequestTopic is the name of the topic from which the service
	// receives transaction requests.
	TransactionRequestTopic string `json:"TRANSACTION_REQUEST_TOPIC"`

	// TransactionStatusTopic is the name of the topic onto which the service
	// places updates about requested transactions.
	TransactionStatusTopic string `json:"TRANSACTION_STATUS_TOPIC"`

	// EthereumRPCURL is the URL of the JSON-RPC endpoint to use for
	// blockchain interactions.
	EthereumRPCURL string `yaml:"ETHEREUM_RPC_URL"`

	// EthereumChainID is the chain ID of the target blockchain. One standard
	// list of these is https://chainlist.org.
	EthereumChainID int `yaml:"ETHEREUM_CHAIN_ID"`

	// PrivateKeyMode is true when the private key for the sender is being injected
	// into the environment. This should never be used in production.
	PrivateKeyMode bool `yaml:"PRIVATE_KEY_MODE"`

	// KMSKeyID is the AWS KMS key id for signing transactions. The KeySpec must be
	// ECC_SECG_P256K1. Only used if PrivateKeyMode is false.
	KMSKeyID string `yaml:"KMS_KEY_ID"`

	// SenderPrivateKey is a hex-encoded private key for the secp256k1 curve, used
	// to sign transactions. Only used if PrivateKeyMode is true.
	SenderPrivateKey string `yaml:"SENDER_PRIVATE_KEY"`

	// ConfirmationBlocks is the number of blocks needed to consider a
	// transaction confirmed.
	ConfirmationBlocks int `yaml:"CONFIRMATION_BLOCKS"`
}
