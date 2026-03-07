package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	AdminID string `json:"admin_id"`
	Level   string `json:"level"`
	jwt.RegisteredClaims
}

// GenerateAccessTokenWithLevel signs HS256 token containing admin_id + level claims.
func GenerateAccessTokenWithLevel(adminID, level string, ttl time.Duration) (string, time.Time, error) {
	secret := accessSecret
	if secret == "" {
		return "", time.Time{}, errors.New("jwt secret is empty")
	}
	exp := time.Now().Add(ttl)
	claims := AccessClaims{
		AdminID: adminID,
		Level:   level,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func ParseAccessTokenClaims(tokenStr string) (*AccessClaims, error) {
	if tokenStr == "" {
		return nil, errors.New("empty token")
	}
	secret := accessSecret
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*AccessClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func GenerateOpaqueRefreshToken() (plain string, hash string, err error) {
	b := make([]byte, 48)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(h[:])
	return plain, hash, nil
}

func HashOpaqueToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}
