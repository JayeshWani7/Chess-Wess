package server

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

// maxRequestBodyBytes is the global limit applied to all non-WebSocket routes.
// 1 MiB is generous for any JSON payload this API accepts.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// cors sets CORS response headers.  When the server has explicit allowed
// origins (from Config.AllowedOrigins) the Origin header is validated;
// otherwise the legacy wildcard "*" is used.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Determine which origin value to echo back.
		allowOrigin := ""
		if len(s.allowedOrigins) == 1 && s.allowedOrigins[0] == "*" {
			allowOrigin = "*"
		} else if origin != "" && isOriginAllowed(origin, s.allowedOrigins) {
			allowOrigin = origin
			w.Header().Set("Vary", "Origin")
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth validates the JWT and injects the user-id into the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		userID, err := validateJWT(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func validateJWT(tokenStr string) (string, error) {
	secret := jwtSecret()
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrTokenInvalidClaims
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return "", err
	}
	return sub, nil
}

// jwtSecret returns the JWT signing secret from the environment.
// LoadConfig() already validates that the secret is present and strong, so
// this function is only called after a successful startup.
func jwtSecret() string {
	return os.Getenv("JWT_SECRET")
}
