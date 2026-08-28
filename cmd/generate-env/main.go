package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// commonServices use the plain "commonRedis" secret (generateSecretsConfig's
// default) - listed explicitly here only because they also gate validity in
// main() below, not because anything picks "redis" for them specifically.
var commonServices = []string{
	"box-service",
	"claw-service",
	"finops-service",
	"listing-service",
	"prereg-onboarding-service",
	"sales-history-service",
	"saved-items-service",
}

var (
	ChallengeServices = []string{
		"edition-upgrade-service",
		"fc-select-service",
		"locking-service",
		"moment-burn-service",
		"moment-leaderboard-service",
		"new-auth-service",
		"packs-trade-service",
		"scout-service",
	}
	SuperteamServices = []string{
		"cm-miscellaneous-service",
		"cm-trade-admin-service",
		"cm-trade-service",
		"cm-trade-stats-service",
		"gift-cards-service",
		"iap-admin-service",
		"iap-service",
		"opensea-leaderboard-service",
		"superteam-album-admin-service",
		"superteam-album-service",
		"superteam-event-admin-service",
		"superteam-event-service",
		"superteam-ipo-admin-service",
		"superteam-ipo-service",
		"superteam-notification-service",
		"superteam-packs-admin-service",
		"superteam-packs-service",
		"superteam-segmentation-service",
		"superteam-shop-service",
		"superteam-transaction-history-service",
		"superteam-user-service",
	}
)

type configItem struct {
	Key       string `json:"key"`
	Env       string `json:"env"`
	ParseJSON bool   `json:"parseJson,omitempty"`
}

func generateSecretsConfig(serviceName string) []configItem {
	items := []configItem{
		{Key: "secretConfig", Env: "secretConfig", ParseJSON: true},
		{Key: "mongodb", Env: "mongodb", ParseJSON: true},
		{Key: "commonRedis", Env: "redis", ParseJSON: true},
		{Key: "postgres", Env: "postgres", ParseJSON: true},
		{Key: "stage", Env: "stage"},
	}

	fmt.Printf("Generating .env for %s service\n", serviceName)

	if slices.Contains(ChallengeServices, serviceName) {
		for i := range items {
			if items[i].Key == "commonRedis" {
				items[i].Key = "challengeRedis"
			}
		}
	}

	if slices.Contains(SuperteamServices, serviceName) {
		for i := range items {
			if items[i].Key == "commonRedis" {
				items[i].Key = "superteamRedis"
			}
		}
	}
	return items
}

func writeEnvFile(serviceName string, items []configItem) error {
	secretsConfig, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	content := fmt.Sprintf(
		"SECRETS_CONFIG='%s'\nENVIRONMENT=dev\nSERVICE_MODE=rest\nIS_LOCAL_DEVELOPMENT=true\nPORT=:8080\nREAD_TIMEOUT=30\nWRITE_TIMEOUT=30\nOTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.observability.svc.cluster.local:4317\nAPM_ENABLED=false\nSERVICENAME=%s\n",
		secretsConfig, serviceName,
	)

	if err := os.WriteFile(".env", []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("Generated .env file")
	return nil
}

func main() {
	serviceName := ""
	if len(os.Args) >= 2 {
		serviceName = os.Args[1]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		serviceName = filepath.Base(cwd)
	}

	valid := slices.Contains(commonServices, serviceName) ||
		slices.Contains(ChallengeServices, serviceName) ||
		slices.Contains(SuperteamServices, serviceName)
	if !valid {
		fmt.Fprintln(os.Stderr, "Error: Invalid serviceName provided")
		os.Exit(1)
	}

	items := generateSecretsConfig(serviceName)
	if err := writeEnvFile(serviceName, items); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
