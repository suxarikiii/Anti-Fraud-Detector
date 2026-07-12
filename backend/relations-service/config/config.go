package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultEnvPath = ".env"

type Config struct {
	Server ServerConfig
	Rabbit RabbitConfig
	Data   DataConfig
}

type ServerConfig struct {
	Port                    string
	GracefulShutdownTimeout time.Duration
}

type RabbitConfig struct {
	Enabled                  bool
	URL                      string
	Exchange                 string
	NormalizedQueue          string
	NormalizedRoutingKey     string
	RelationsBuiltRoutingKey string
}

type DataConfig struct {
	DatasetID           string
	DatasetPath         string
	DemoFallbackEnabled bool
}

func Load() (*Config, error) {
	if err := loadDotEnv(defaultEnvPath); err != nil {
		return nil, err
	}

	return &Config{
		Server: ServerConfig{
			Port:                    getEnv("SERVER_PORT", ":8082"),
			GracefulShutdownTimeout: getDurationEnv("GRACEFUL_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Rabbit: RabbitConfig{
			Enabled:                  getBoolEnv("RABBITMQ_ENABLED", false),
			URL:                      getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			Exchange:                 getEnv("RABBITMQ_EXCHANGE", "pipeline.exchange"),
			NormalizedQueue:          getEnv("RABBITMQ_NORMALIZED_QUEUE", "relations.dataset-normalized.queue"),
			NormalizedRoutingKey:     getEnv("RABBITMQ_NORMALIZED_ROUTING_KEY", "dataset.normalized"),
			RelationsBuiltRoutingKey: getEnv("RABBITMQ_RELATIONS_BUILT_ROUTING_KEY", "refund.relations.built"),
		},
		Data: DataConfig{
			DatasetID:           getEnv("RELATIONS_DATASET_ID", "demo"),
			DatasetPath:         getEnv("RELATIONS_DATASET_PATH", ""),
			DemoFallbackEnabled: getBoolEnv("RELATIONS_DEMO_FALLBACK_ENABLED", true),
		},
	}, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNumber)
		}

		key = strings.TrimSpace(key)
		value = cleanEnvValue(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty env key", path, lineNumber)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: set %s: %w", path, lineNumber, key, err)
		}
	}

	return scanner.Err()
}

func cleanEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}

	if before, _, ok := strings.Cut(value, " #"); ok {
		return strings.TrimSpace(before)
	}

	return value
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getBoolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
		return duration
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}
