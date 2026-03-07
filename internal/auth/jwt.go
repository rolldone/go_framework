package auth

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	accessSecret  string
	refreshSecret string
	accessExp     = int(getEnvInt("JWT_ACCESS_EXP_SECONDS", 600))      // 10m
	refreshExp    = int(getEnvInt("JWT_REFRESH_EXP_SECONDS", 1209600)) // 14d
)

func init() {
	// Prefer a single canonical secret if provided
	if s := os.Getenv("AUTH_JWT_SECRET"); s != "" {
		accessSecret = s
		refreshSecret = s
		return
	}

	// Fallback to legacy separate secrets for compatibility
	accessSecret = getEnv("JWT_ACCESS_SECRET", "change-me-access-secret")
	refreshSecret = getEnv("JWT_REFRESH_SECRET", "change-me-refresh-secret")
}

// AccessExpirySeconds returns configured access token lifetime in seconds.
func AccessExpirySeconds() int {
	return accessExp
}

// RefreshExpirySeconds returns configured refresh token lifetime in seconds.
func RefreshExpirySeconds() int {
	return refreshExp
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// SignAccessToken creates a signed JWT access token for the given subject (user id).
func SignAccessToken(sub string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": sub,
		"iat": now.Unix(),
		"exp": now.Add(time.Duration(accessExp) * time.Second).Unix(),
		"typ": "access",
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(accessSecret))
}

// SignRefreshToken creates a signed JWT refresh token for the given subject (user id).
func SignRefreshToken(sub string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": sub,
		"iat": now.Unix(),
		"exp": now.Add(time.Duration(refreshExp) * time.Second).Unix(),
		"typ": "refresh",
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(refreshSecret))
}

// ParseAccessToken verifies and returns the subject from an access token.
func ParseAccessToken(tokenStr string) (string, error) {
	return parseTokenWithSecret(tokenStr, accessSecret, "access")
}

// ParseRefreshToken verifies and returns the subject from a refresh token.
func ParseRefreshToken(tokenStr string) (string, error) {
	return parseTokenWithSecret(tokenStr, refreshSecret, "refresh")
}

func parseTokenWithSecret(tokenStr, secret, expectType string) (string, error) {
	if tokenStr == "" {
		return "", errors.New("empty token")
	}
	parser := jwt.NewParser()
	tok, err := parser.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	if !tok.Valid {
		return "", errors.New("invalid token")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}
	if p, ok := claims["typ"]; !ok || p != expectType {
		return "", errors.New("invalid token type")
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing subject")
	}
	return sub, nil
}
