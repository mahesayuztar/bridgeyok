package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment             string
	Host                    string
	Port                    int
	DatabaseURL             string
	DatabaseMaxConns        int32
	TableActorQueueCapacity int
	TableActorIdleTimeout   time.Duration
	AuthSecret              []byte
	AllowedOrigins          []string
	LogLevel                slog.Level
	ReadHeaderTimeout       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
	ShutdownTimeout         time.Duration
}

type lookupFunc func(string) (string, bool)

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func (config Config) Address() string {
	return net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
}

func load(lookup lookupFunc) (Config, error) {
	environment := valueOrDefault(lookup, "APP_ENV", "development")
	if environment != "development" && environment != "test" && environment != "preview" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, preview, or production")
	}

	databaseURL, ok := lookup("DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if !validDatabaseURL(databaseURL) {
		return Config{}, fmt.Errorf("DATABASE_URL must be a valid PostgreSQL URL")
	}

	port, err := integerValue(lookup, "PORT", 8080, 1, 65535)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConns, err := integerValue(lookup, "DATABASE_MAX_CONNS", 5, 1, 50)
	if err != nil {
		return Config{}, err
	}
	tableActorQueueCapacity, err := integerValue(lookup, "TABLE_ACTOR_QUEUE_CAPACITY", 64, 1, 1024)
	if err != nil {
		return Config{}, err
	}
	tableActorIdleTimeout, err := durationValue(lookup, "TABLE_ACTOR_IDLE_TIMEOUT", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	authSecret, ok := lookup("AUTH_SECRET")
	if !ok || len(authSecret) < 32 {
		return Config{}, fmt.Errorf("AUTH_SECRET must contain at least 32 characters")
	}
	allowedOrigins, err := originValues(lookup, environment)
	if err != nil {
		return Config{}, err
	}
	logLevel, err := logLevelValue(lookup)
	if err != nil {
		return Config{}, err
	}
	readHeaderTimeout, err := durationValue(lookup, "READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := durationValue(lookup, "READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := durationValue(lookup, "WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := durationValue(lookup, "IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationValue(lookup, "SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Environment:             environment,
		Host:                    valueOrDefault(lookup, "API_HOST", "0.0.0.0"),
		Port:                    port,
		DatabaseURL:             databaseURL,
		DatabaseMaxConns:        int32(databaseMaxConns),
		TableActorQueueCapacity: tableActorQueueCapacity,
		TableActorIdleTimeout:   tableActorIdleTimeout,
		AuthSecret:              []byte(authSecret),
		AllowedOrigins:          allowedOrigins,
		LogLevel:                logLevel,
		ReadHeaderTimeout:       readHeaderTimeout,
		ReadTimeout:             readTimeout,
		WriteTimeout:            writeTimeout,
		IdleTimeout:             idleTimeout,
		ShutdownTimeout:         shutdownTimeout,
	}, nil
}

func valueOrDefault(lookup lookupFunc, key string, fallback string) string {
	if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func integerValue(lookup lookupFunc, key string, fallback int, minimum int, maximum int) (int, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func durationValue(lookup lookupFunc, key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func logLevelValue(lookup lookupFunc) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(valueOrDefault(lookup, "LOG_LEVEL", "info"))); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	return level, nil
}

func originValues(lookup lookupFunc, environment string) ([]string, error) {
	raw, ok := lookup("ALLOWED_ORIGINS")
	if !ok || strings.TrimSpace(raw) == "" {
		if environment == "development" || environment == "test" {
			return []string{"http://localhost:3000"}, nil
		}
		return nil, fmt.Errorf("ALLOWED_ORIGINS is required outside development and test")
	}

	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for rawOrigin := range strings.SplitSeq(raw, ",") {
		origin, err := normalizeOrigin(rawOrigin)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("ALLOWED_ORIGINS must contain at least one exact HTTP origin")
	}
	return origins, nil
}

func normalizeOrigin(raw string) (string, error) {
	origin := strings.TrimSpace(raw)
	if origin == "*" {
		return "", fmt.Errorf("ALLOWED_ORIGINS cannot contain a wildcard")
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("ALLOWED_ORIGINS must contain exact HTTP origins without paths, credentials, queries, or fragments")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func validDatabaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") && parsed.Host != "" && parsed.Path != ""
}
