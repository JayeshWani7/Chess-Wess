package server

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_Origins(t *testing.T) {
	// Backup environment variables
	origAppEnv := os.Getenv("APP_ENV")
	origEnv := os.Getenv("ENV")
	origAllowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	origDevPermissive := os.Getenv("DEV_PERMISSIVE_CORS")
	origJWT := os.Getenv("JWT_SECRET")
	origDB := os.Getenv("DATABASE_URL")

	// Set required vars for config to not fail on other things
	os.Setenv("JWT_SECRET", "this-is-a-very-long-secret-greater-than-32-chars")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")

	defer func() {
		// Restore environment variables
		os.Setenv("APP_ENV", origAppEnv)
		os.Setenv("ENV", origEnv)
		os.Setenv("ALLOWED_ORIGINS", origAllowedOrigins)
		os.Setenv("DEV_PERMISSIVE_CORS", origDevPermissive)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("DATABASE_URL", origDB)
	}()

	tests := []struct {
		name           string
		envAppEnv      string
		envAllowed     string
		envDevPerm     string
		expectError    bool
		errorContains  string
		expectedLength int
		expectWildcard bool
	}{
		{
			name:           "Production fails with empty allowed origins",
			envAppEnv:      "production",
			envAllowed:     "",
			expectError:    true,
			errorContains:  "ALLOWED_ORIGINS must be explicitly configured in production environment",
		},
		{
			name:           "Production fails with wildcard allowed origin",
			envAppEnv:      "production",
			envAllowed:     "*",
			expectError:    true,
			errorContains:  "production cannot start with unsafe wildcard origin '*'",
		},
		{
			name:           "Production fails with wildcard in list",
			envAppEnv:      "production",
			envAllowed:     "https://chesswess.com, *",
			expectError:    true,
			errorContains:  "production cannot start with unsafe wildcard origin '*'",
		},
		{
			name:           "Production fails with wildcard character",
			envAppEnv:      "production",
			envAllowed:     "https://*.chesswess.com",
			expectError:    true,
			errorContains:  "cannot contain wildcard characters",
		},
		{
			name:           "Production fails with invalid URL (no scheme)",
			envAppEnv:      "production",
			envAllowed:     "chesswess.com",
			expectError:    true,
			errorContains:  "must be a valid absolute URL",
		},
		{
			name:           "Production succeeds with valid origins",
			envAppEnv:      "production",
			envAllowed:     "https://chesswess.com, https://api.chesswess.com",
			expectError:    false,
			expectedLength: 2,
		},
		{
			name:           "Development defaults to safe local origins",
			envAppEnv:      "development",
			envAllowed:     "",
			expectError:    false,
			expectedLength: 5,
		},
		{
			name:           "Development allows wildcard when DEV_PERMISSIVE_CORS is true",
			envAppEnv:      "development",
			envAllowed:     "",
			envDevPerm:     "true",
			expectError:    false,
			expectedLength: 1,
			expectWildcard: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("APP_ENV", tt.envAppEnv)
			os.Setenv("ALLOWED_ORIGINS", tt.envAllowed)
			os.Setenv("DEV_PERMISSIVE_CORS", tt.envDevPerm)

			cfg, err := LoadConfig()
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error but got: %v", err)
				}
				if len(cfg.AllowedOrigins) != tt.expectedLength {
					t.Errorf("expected %d allowed origins, got %d: %v", tt.expectedLength, len(cfg.AllowedOrigins), cfg.AllowedOrigins)
				}
				if tt.expectWildcard && cfg.AllowedOrigins[0] != "*" {
					t.Errorf("expected wildcard '*' but got %v", cfg.AllowedOrigins)
				}
			}
		})
	}
}
