package config

import (
	"fmt"
	"github.com/goccy/go-json"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func Get(key string) interface{} {
	return viper.Get(key)
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetBool(key string) bool {
	return viper.GetBool(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
}

func GetMap(key string) map[string]interface{} {
	return viper.GetStringMap(key)
}

func GetStringMap(key string) map[string]string {
	return viper.GetStringMapString(key)
}

func GetSlice(key string) []string {
	return viper.GetStringSlice(key)
}

func Init() {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envVar := "CONFIG"
		jsonConfig := os.Getenv(envVar)
		if jsonConfig == "" {
			panic(fmt.Errorf("environment variable %s not set", envVar))
		}
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(jsonConfig), &configMap); err != nil {
			panic(fmt.Errorf("failed to unmarshal JSON from environment variable: %w", err))
		}
		viper.SetConfigType("json")
		if err := viper.MergeConfigMap(configMap); err != nil {
			panic(fmt.Errorf("failed to merge config map into Viper: %w", err))
		}
	} else {
		reader, err := os.Open(fmt.Sprintf("./config/%s.json", envFile))
		if err != nil {
			panic(fmt.Errorf("unable to read config file\n %w", err))
		}
		viper.SetConfigType("json")
		if err := viper.MergeConfig(reader); err != nil {
			panic(fmt.Errorf("failed to merge config map into Viper: %w", err))
		}
	}
}

func MergeConfigFromFilePath(key string, path string) {
	viper.SetConfigType("json")
	reader, err := os.Open(path)
	if err != nil {
		panic(fmt.Errorf("unable to read config file\n %w", err))
	}

	var configMap map[string]interface{}
	err = json.NewDecoder(reader).Decode(&configMap)
	if err != nil {
		panic(fmt.Errorf("unable to decode config file\n %w", err))
	}
	viper.Set(key, configMap)
}

func formatEnvKeys(envData []byte) []byte {
	formattedEnv := make([]byte, 0, len(envData))
	data := strings.Split(string(envData), "\n")
	for _, line := range data {
		if line == "" {
			continue
		}
		splits := strings.SplitN(line, "=", 2)
		newKKey := strings.ReplaceAll(splits[0], "_", ".")
		formattedEnv = append(formattedEnv, []byte(newKKey+"="+splits[1]+"\n")...)
	}
	return formattedEnv
}
