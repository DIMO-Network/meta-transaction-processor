package config

import (
	"github.com/DIMO-Network/shared/db"
)

type Settings struct {
	// MonitoringPort is the port on which we run the health check endpoint and
	// serve Prometheus metrics.
	MonitoringPort string `yaml:"MONITORING_PORT"`

	// KafkaServers is a comma-seperated list of Kafka bootstrap servers.
	// We typically only specify one.
	KafkaServers string `yaml:"KAFKA_SERVERS"`

	// ConsumerGroupName is the name of the consumer group.

	// TransactionRequestTopic is the name of the topic from which the service
	// receives transaction requests.
	TransactionRequestTopic string `yaml:"TRANSACTION_REQUEST_TOPIC"`

	// TransactionStatusTopic is the name of the topic onto which the service
	// places updates about requested transactions.
	TransactionStatusTopic string `yaml:"TRANSACTION_STATUS_TOPIC"`

	// EthereumRPCURL is the URL of the JSON-RPC endpoint to use for
	// blockchain interactions.
	EthereumRPCURL string `yaml:"ETHEREUM_RPC_URL"`

	// PrivateKeyMode is true when the private key for the sender is being injected
	// into the environment. This should never be used in production.
	PrivateKeyMode bool `yaml:"PRIVATE_KEY_MODE"`

	// KMSKeyID is the AWS KMS key id for signing transactions. The KeySpec must be
	// ECC_SECG_P256K1. Only used if PrivateKeyMode is false.
	KMSKeyID string `yaml:"KMS_KEY_ID"`

	// KMSKeyIDs is a comma-separated list of AWS KMS key ids for signing transactions.
	// The KeySpec must be ECC_SECG_P256K1. Only used if PrivateKeyMode is false.
	KMSKeyIDs string `yaml:"KMS_KEY_IDS"`

	// SenderPrivateKey is a hex-encoded private key for the secp256k1 curve, used
	// to sign transactions. Only used if PrivateKeyMode is true.
	SenderPrivateKey string `yaml:"SENDER_PRIVATE_KEY"`

	// SenderPrivateKeys is a hex-encoded private key for the secp256k1 curve, used
	// to sign transactions. Only used if PrivateKeyMode is true.
	SenderPrivateKeys string `yaml:"SENDER_PRIVATE_KEYS"`

	// ConfirmationBlocks is the number of blocks needed to consider a
	// transaction confirmed.
	ConfirmationBlocks int64 `yaml:"CONFIRMATION_BLOCKS"`

	BoostAfterBlocks int64 `yaml:"BOOST_AFTER_BLOCKS"`

	DB db.Settings `yaml:"DB"`

	// Average block time, in seconds.
	BlockTime int `yaml:"BLOCK_TIME"`

	GRPCPort string `yaml:"GRPC_PORT"`

	AWSEndpoint string `yaml:"AWS_ENDPOINT"`
	AWSRegion   string `yaml:"AWS_REGION"`

	DisableBoosting bool `yaml:"DISABLE_BOOSTING"`
}
