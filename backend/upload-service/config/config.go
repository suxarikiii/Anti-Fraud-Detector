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
	DB     DBConfig
	Rabbit RabbitConfig
	MinIO  MinioConfig
	Upload UploadConfig
	Admin  AdminConfig
}

type ServerConfig struct {
	Port                    string
	GracefulShutdownTimeout time.Duration
	ReadHeaderTimeout       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	URL      string
}

type RabbitConfig struct {
	URL          string
	Exchange     string
	EventsQueue  string
	DeadQueue    string
	ConsumerName string
	MaxRetries   int
}

type UploadConfig struct {
	MaxFileSize int64
	MaxRows     int
	MaxErrors   int
}

type AdminConfig struct {
	StatusPatchEnabled bool
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

func Load() (*Config, error) {
	if err := loadDotEnv(defaultEnvPath); err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:                    getEnv("SERVER_PORT", ":8081"),
			GracefulShutdownTimeout: getDurationEnv("GRACEFUL_SHUTDOWN_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout:       getDurationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:             getDurationEnv("HTTP_READ_TIMEOUT", 2*time.Minute),
			WriteTimeout:            getDurationEnv("HTTP_WRITE_TIMEOUT", 2*time.Minute),
			IdleTimeout:             getDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "upload_db"),
		},
		Rabbit: RabbitConfig{
			URL:          getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			Exchange:     getEnv("RABBITMQ_EXCHANGE", "pipeline.exchange"),
			EventsQueue:  getEnv("RABBITMQ_UPLOAD_EVENTS_QUEUE", "upload.pipeline-events.queue"),
			DeadQueue:    getEnv("RABBITMQ_UPLOAD_DLQ", "upload.pipeline-events.dlq"),
			ConsumerName: getEnv("RABBITMQ_UPLOAD_CONSUMER", "upload-service"),
			MaxRetries:   getIntEnv("RABBITMQ_MAX_RETRIES", 3),
		},
		MinIO: MinioConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("MINIO_BUCKET", "datasets"),
			Secure:    getBoolEnv("MINIO_SECURE", false),
		},
		Upload: UploadConfig{
			MaxFileSize: int64(getIntEnv("UPLOAD_MAX_FILE_SIZE_BYTES", 50<<20)),
			MaxRows:     getIntEnv("UPLOAD_MAX_ROWS", 250000),
			MaxErrors:   getIntEnv("UPLOAD_MAX_VALIDATION_ERRORS", 100),
		},
		Admin: AdminConfig{StatusPatchEnabled: getBoolEnv("ADMIN_STATUS_PATCH_ENABLED", false)},
	}

	cfg.DB.URL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	return cfg, nil
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
