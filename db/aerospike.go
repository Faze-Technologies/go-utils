package db

import (
	"crypto/tls"
	"time"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/aerospike/aerospike-client-go/v6"
	"go.uber.org/zap"
)

func InitAerospikeDB() *aerospike.Client {
	logger := logs.GetLogger()

	// Get configuration values for Kubernetes deployment
	host := config.GetString("aerospike.host")
	port := config.GetInt("aerospike.port")

	// Set default port if not specified (standard Aerospike port)
	if port == 0 {
		port = 3000 // Default Aerospike port
	}

	logger.Info("Connecting to Aerospike Kubernetes Cluster",
		zap.String("host", host),
		zap.Int("port", port))

	// Create Aerospike client policy optimized for K8s
	clientPolicy := aerospike.NewClientPolicy()

	// Set connection timeout suitable for K8s environment
	timeoutSeconds := config.GetInt("aerospike.timeout_seconds")
	if timeoutSeconds > 0 {
		clientPolicy.Timeout = time.Duration(timeoutSeconds) * time.Second
	}
	// Default timeout is already set in clientPolicy

	// Configure connection pool for K8s deployment
	clientPolicy.ConnectionQueueSize = config.GetInt("aerospike.connection_queue_size")
	if clientPolicy.ConnectionQueueSize == 0 {
		clientPolicy.ConnectionQueueSize = 256 // Default for K8s deployment
	}

	// Optional: Configure authentication for Aerospike Community Edition
	// Note: Community Edition has limited auth features compared to Enterprise
	username := config.GetString("aerospike.username")
	password := config.GetString("aerospike.password")
	if username != "" && password != "" {
		clientPolicy.User = username
		clientPolicy.Password = password
		logger.Info("Using authentication for Aerospike connection")
	}

	// Optional: Configure TLS if enabled in K8s operator
	tlsName := config.GetString("aerospike.tls_name")
	enableTLS := config.GetBool("aerospike.enable_tls")
	if enableTLS && tlsName != "" {
		clientPolicy.TlsConfig = &tls.Config{
			ServerName:         tlsName,
			InsecureSkipVerify: config.GetBool("aerospike.tls_skip_verify"), // For dev environments
		}
		logger.Info("Using TLS for Aerospike connection", zap.String("tls_name", tlsName))
	}

	// Create the client connection
	client, err := aerospike.NewClientWithPolicy(clientPolicy, host, port)
	if err != nil {
		logger.Fatal("Error connecting to Aerospike K8s cluster", zap.Error(err))
		return nil
	}

	// Test the connection
	if !client.IsConnected() {
		logger.Fatal("Failed to establish connection to Aerospike K8s cluster")
		return nil
	}

	logger.Info("Successfully connected to Aerospike Kubernetes cluster!")

	return client
}

func CloseAerospikeDB(client *aerospike.Client) {
	if client != nil {
		client.Close()
	}
}
