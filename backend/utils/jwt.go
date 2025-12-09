package utils

import (
	"time"

	"mtv2/backend/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	IPAddress    string `json:"ip_address"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, username, ipAddress string, tokenVersion int) (string, error) {
	claims := Claims{
		UserID:       userID,
		Username:     username,
		IPAddress:    ipAddress,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.Server.JWTSecret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.Server.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
