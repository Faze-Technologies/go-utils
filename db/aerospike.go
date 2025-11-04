package db

import (
	"crypto/tls"
	"strconv"
	"strings"
	"time"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/aerospike/aerospike-client-go/v6"
	"go.uber.org/zap"
)

func InitAerospikeDB() *aerospike.Client {
	logger := logs.GetLogger()

	// Get configuration values
	port := config.GetInt("aerospike.port")

	// Set default port if not specified (standard Aerospike port)
	if port == 0 {
		port = 3000 // Default Aerospike port
	}

	logger.Info("Connecting to Aerospike cluster", zap.Int("port", port))

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

	// Expect only array form in config: `aerospike.hosts: ["ip1","ip2"]`
	hostCandidates := []string{}
	hostSlice := config.GetSlice("aerospike.hosts")
	if len(hostSlice) == 0 {
		logger.Fatal("`aerospike.hosts` must be provided as an array of host addresses")
		return nil
	}
	for _, hh := range hostSlice {
		h := strings.TrimSpace(hh)
		if h == "" {
			continue
		}
		if !strings.Contains(h, ":") {
			h = h + ":" + strconv.Itoa(port)
		}
		hostCandidates = append(hostCandidates, h)
	}

	if len(hostCandidates) == 0 {
		logger.Fatal("No Aerospike hosts provided in configuration")
		return nil
	}

	// Create aerospike.Host instances using NewHosts and let the client do
	// cluster discovery. This is the recommended approach: pass seed-list to
	// the client (it will discover the rest of the cluster).
	logger.Info("Creating Aerospike seed hosts", zap.Strings("hosts", hostCandidates))
	hosts, herr := aerospike.NewHosts(hostCandidates...)
	if herr != nil {
		logger.Fatal("Failed to parse Aerospike host addresses", zap.Error(herr))
		return nil
	}

	client, err := aerospike.NewClientWithPolicyAndHost(clientPolicy, hosts...)
	if err != nil {
		logger.Fatal("Error creating Aerospike client with seed hosts", zap.Error(err))
		return nil
	}

	if !client.IsConnected() {
		client.Close()
		logger.Fatal("Failed to establish connection to Aerospike cluster using provided seed hosts")
		return nil
	}

	logger.Info("Successfully connected to Aerospike cluster via seed hosts")
	return client
}

func CloseAerospikeDB(client *aerospike.Client) {
	if client != nil {
		client.Close()
	}
}
