package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

var commonServices = []string{}

var (
	ChallengeServices = []string{}
	SuperteamServices = []string{"cm-miscellaneous-service"}
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

	server, err := json.Marshal(map[string]interface{}{
		"port":         ":8080",
		"readTimeout":  5,
		"writeTimeout": 5,
	})
	if err != nil {
		return err
	}

	apm, err := json.Marshal(map[string]interface{}{
		"enabled":      false,
		"serviceName":  serviceName,
		"otlpEndpoint": "otel-collector.observability.svc.cluster.local:4317",
	})
	if err != nil {
		return err
	}

	content := fmt.Sprintf(
		"SECRETS_CONFIG='%s'\nENVIRONMENT=dev\nSERVICE_MODE=rest\nIS_LOCAL_DEVELOPMENT=true\nSERVER='%s'\nAPM='%s'\n",
		secretsConfig, server, apm,
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
