package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

var commonServices = []string{
	"analytics-service",
	"analytics-service-pubsub",
	"blockchain-request-hook-service",
	"blockchain-request-queue-service",
	"blockchain-request-service",
	"bff-service",
	"cms-service",
	"control-service",
	"cron-scheduler-service",
	"event-service",
	"finance-service",
	"finance-service-pubsub",
	"payment-gateway-service",
	"payment-gateway-service-pubsub",
	"flow-service",
	"flow-service-pubsub",
	"freshdesk-service",
	"kyc-service",
	"milestone-service",
	"milestone-service-pubsub",
	"mini-games-service",
	"mini-games-service-pubsub",
	"nba-data-service",
	"nba-data-service-pubsub",
	"nba-flash-service",
	"nba-flash-service-pubsub",
	"newadmin-bff",
	"notification-service",
	"notification-service-pubsub",
	"notification-service-v2",
	"notification-service-v2-pubsub",
	"oldadmin-bff",
	"packs-service",
	"packs-service-pubsub",
	"personalization-service",
	"research-service",
	"research-service-pubsub",
	"reward-service",
	"reward-service-pubsub",
	"risk-analysis-service",
	"risk-analysis-service-pubsub",
	"segmentation-service",
	"set-service",
	"set-service-pubsub",
	"team-service",
	"wallet-service",
	"wallet-service-pubsub",
	"boiler-service",
}

var challengeServices = []string{
	"new-auth-service",
	"challenge-service",
	"challenge-service-pubsub",
	"leaderboard-service",
	"mint-factory-service",
	"moment-service",
	"moment-service-pubsub",
	"nft-service",
	"profile-service",
	"profile-service-pubsub",
	"sports-service",
	"sports-service-pubsub",
}

var fandomServices = []string{
	"fandom-moment-service",
	"fandom-moment-service-pubsub",
	"fandom-finance-service",
	"fandom-finance-service-pubsub",
}

var paymentGatewayServices = []string{
	"payment-gateway-service",
	"payment-gateway-service-pubsub",
}

type configItem struct {
	Key       string `json:"key"`
	Env       string `json:"env"`
	ParseJSON bool   `json:"parseJson,omitempty"`
}

func generateSecretsConfig(serviceName string) []configItem {
	config := []configItem{
		{Key: "secretConfig", Env: "secretConfig", ParseJSON: true},
		{Key: "mongodb", Env: "mongodb", ParseJSON: true},
		{Key: "commonRedis", Env: "redis", ParseJSON: true},
		{Key: "postgres", Env: "postgres", ParseJSON: true},
		{Key: "stage", Env: "stage"},
	}

	fmt.Printf("Generating .env for %s service\n", serviceName)

	if slices.Contains(challengeServices, serviceName) {
		for i := range config {
			if config[i].Key == "commonRedis" {
				config[i].Key = "challengeRedis"
			}
		}
	}

	if slices.Contains(fandomServices, serviceName) {
		for i := range config {
			if config[i].Key == "commonRedis" {
				config[i].Key = "fandomRedis"
			}
		}
	}

	if serviceName == "cron-scheduler-service" || serviceName == "newadmin-bff" {
		fmt.Printf("Adding challengeRedis for %s\n", serviceName)
		config = append(config, configItem{Key: "challengeRedis", Env: "challengeRedis", ParseJSON: true})
	}

	if slices.Contains(paymentGatewayServices, serviceName) {
		config = append(config, configItem{Key: "dek", Env: "dek", ParseJSON: true})
	}

	return config
}

func writeEnvFile(config []configItem) error {
	secretsConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	content := fmt.Sprintf("SECRETS_CONFIG='%s'", secretsConfig)

	if err := os.WriteFile(".env", []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("Generated .env file")
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: serviceName must be provided")
		os.Exit(1)
	}
	serviceName := os.Args[1]

	valid := slices.Contains(commonServices, serviceName) ||
		slices.Contains(challengeServices, serviceName) ||
		slices.Contains(fandomServices, serviceName)
	if !valid {
		fmt.Fprintln(os.Stderr, "Error: Invalid serviceName provided")
		os.Exit(1)
	}

	config := generateSecretsConfig(serviceName)
	if err := writeEnvFile(config); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
