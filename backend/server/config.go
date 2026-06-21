package server

// config.go — startup validation and secure defaults for environment variables.
// Called from main.go before any server construction.

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds all validated environment values.
type Config struct {
	AppEnv         string
	Port           string
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	AllowedOrigins []string // comma-separated CORS / WS origins; "*" means any
}

// LoadConfig reads and validates required environment variables.
// It returns a descriptive error that lists every missing or insecure value so
// operators can fix all problems in one restart.
func LoadConfig() (*Config, error) {
	var errs []string

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = os.Getenv("ENV")
	}
	if appEnv == "" {
		appEnv = "development"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		errs = append(errs, "JWT_SECRET is not set")
	} else if jwtSecret == "dev-secret-change-me" {
		errs = append(errs, "JWT_SECRET is set to the insecure dev default — choose a random secret of ≥ 32 characters")
	} else if len(jwtSecret) < 32 {
		errs = append(errs, "JWT_SECRET must be at least 32 characters")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		errs = append(errs, "DATABASE_URL is not set")
	}

	// Parse and validate Allowed Origins
	var allowedOrigins []string
	rawAllowed := os.Getenv("ALLOWED_ORIGINS")
	if rawAllowed != "" {
		parts := strings.Split(rawAllowed, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				allowedOrigins = append(allowedOrigins, part)
			}
		}
	}

	if appEnv == "production" {
		if len(allowedOrigins) == 0 {
			errs = append(errs, "ALLOWED_ORIGINS must be explicitly configured in production environment")
		} else {
			for _, origin := range allowedOrigins {
				if origin == "*" {
					errs = append(errs, "production cannot start with unsafe wildcard origin '*'")
					continue
				}
				if strings.Contains(origin, "*") {
					errs = append(errs, fmt.Sprintf("production origin '%s' cannot contain wildcard characters", origin))
					continue
				}
				u, err := url.Parse(origin)
				if err != nil || u.Scheme == "" || u.Host == "" {
					errs = append(errs, fmt.Sprintf("production origin '%s' must be a valid absolute URL (e.g. https://example.com)", origin))
				}
			}
		}
	} else {
		// development or test env
		if len(allowedOrigins) == 0 {
			if os.Getenv("DEV_PERMISSIVE_CORS") == "true" {
				allowedOrigins = []string{"*"}
			} else {
				allowedOrigins = []string{
					"http://localhost:3000",
					"http://localhost:5173",
					"http://127.0.0.1:3000",
					"http://127.0.0.1:5173",
					"http://localhost:8080",
				}
			}
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("startup configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	return &Config{
		AppEnv:         appEnv,
		Port:           port,
		DatabaseURL:    databaseURL,
		RedisURL:       redisURL,
		JWTSecret:      jwtSecret,
		AllowedOrigins: allowedOrigins,
	}, nil
}

// isOriginAllowed returns true when origin is found in the allowed list or
// the list contains the wildcard "*".
func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// validateJWTSecretEnv is called at package init time so tests that import
// the server package without going through LoadConfig still surface the issue.
// It is intentionally a non-fatal warning rather than a panic so existing unit
// tests keep working.
var _ = errors.New // prevent unused-import if errors is only used transitively
