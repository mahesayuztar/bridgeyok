package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		values     map[string]string
		wantErr    string
		assertions func(*testing.T, Config)
	}{
		{
			name: "development defaults",
			values: map[string]string{
				"DATABASE_URL": "postgresql://bridgeyok:secret@localhost:5432/bridgeyok?sslmode=disable",
			},
			assertions: func(t *testing.T, config Config) {
				t.Helper()
				if config.Address() != "0.0.0.0:8080" {
					t.Fatalf("Address() = %q, want %q", config.Address(), "0.0.0.0:8080")
				}
				if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "http://localhost:3000" {
					t.Fatalf("AllowedOrigins = %v, want local web origin", config.AllowedOrigins)
				}
				if config.ShutdownTimeout != 10*time.Second {
					t.Fatalf("ShutdownTimeout = %s, want 10s", config.ShutdownTimeout)
				}
			},
		},
		{
			name: "production exact origins",
			values: map[string]string{
				"APP_ENV":          "production",
				"DATABASE_URL":     "postgresql://bridgeyok:secret@db.example.com:5432/bridgeyok?sslmode=require",
				"ALLOWED_ORIGINS":  "https://bridgeyok.example, https://www.bridgeyok.example/",
				"PORT":             "10000",
				"LOG_LEVEL":        "warn",
				"SHUTDOWN_TIMEOUT": "20s",
			},
			assertions: func(t *testing.T, config Config) {
				t.Helper()
				if config.Port != 10000 || len(config.AllowedOrigins) != 2 || config.ShutdownTimeout != 20*time.Second {
					t.Fatalf("unexpected production config: %+v", config)
				}
			},
		},
		{
			name:    "missing database",
			values:  map[string]string{},
			wantErr: "DATABASE_URL is required",
		},
		{
			name: "short auth secret",
			values: map[string]string{
				"DATABASE_URL": "postgresql://bridgeyok:secret@localhost:5432/bridgeyok",
				"AUTH_SECRET":  "too-short",
			},
			wantErr: "AUTH_SECRET must contain at least 32 characters",
		},
		{
			name: "invalid database",
			values: map[string]string{
				"DATABASE_URL": "not-a-url",
			},
			wantErr: "valid PostgreSQL URL",
		},
		{
			name: "production missing origin",
			values: map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": "postgresql://bridgeyok:secret@db.example.com:5432/bridgeyok",
			},
			wantErr: "ALLOWED_ORIGINS is required",
		},
		{
			name: "wildcard origin",
			values: map[string]string{
				"DATABASE_URL":    "postgresql://bridgeyok:secret@localhost:5432/bridgeyok",
				"ALLOWED_ORIGINS": "*",
			},
			wantErr: "cannot contain a wildcard",
		},
		{
			name: "origin with path",
			values: map[string]string{
				"DATABASE_URL":    "postgresql://bridgeyok:secret@localhost:5432/bridgeyok",
				"ALLOWED_ORIGINS": "https://bridgeyok.example/app",
			},
			wantErr: "exact HTTP origins",
		},
		{
			name: "invalid port",
			values: map[string]string{
				"DATABASE_URL": "postgresql://bridgeyok:secret@localhost:5432/bridgeyok",
				"PORT":         "70000",
			},
			wantErr: "PORT must be an integer",
		},
		{
			name: "invalid timeout",
			values: map[string]string{
				"DATABASE_URL": "postgresql://bridgeyok:secret@localhost:5432/bridgeyok",
				"READ_TIMEOUT": "0s",
			},
			wantErr: "READ_TIMEOUT must be a positive duration",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config, err := load(mapLookup(test.values))
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("load() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load() unexpected error: %v", err)
			}
			if test.assertions != nil {
				test.assertions(t, config)
			}
		})
	}
}

func TestLoadDoesNotExposeDatabaseCredentials(t *testing.T) {
	t.Parallel()

	credential := "private-password"
	_, err := load(mapLookup(map[string]string{
		"DATABASE_URL": "postgresql://bridgeyok:" + credential + "@/invalid",
	}))
	if err == nil {
		t.Fatal("load() expected an error")
	}
	if strings.Contains(err.Error(), credential) {
		t.Fatalf("load() exposed database credentials: %v", err)
	}
}

func mapLookup(values map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		if key == "AUTH_SECRET" {
			if value, ok := values[key]; ok {
				return value, true
			}
			return strings.Repeat("test-secret-", 3), true
		}
		value, ok := values[key]
		return value, ok
	}
}
