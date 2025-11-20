package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid            = errors.New("token is invalid")
	ErrTokenExpired            = errors.New("token has expired")
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	ErrMissingClaims           = errors.New("required claims are missing")
)

type JWTManager struct {
	secretKey     string
	tokenDuration *time.Duration
	issuer        string
}

type AuthClaims struct {
	PhoneNumber string `json:"phone_number"`
	jwt.RegisteredClaims
}

func NewJWTManager(secretKey, issuer string, tokenDuration *time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: tokenDuration,
		issuer:        issuer,
	}
}

func (m *JWTManager) GenerateTokens(phoneNumber string) (token string, err error) {
	claims := AuthClaims{
		PhoneNumber: phoneNumber,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   m.issuer,
			Subject:  phoneNumber,
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	if m.tokenDuration != nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(*m.tokenDuration))
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(m.secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return token, nil
}

func (m *JWTManager) ValidateToken(tokenString string) (*AuthClaims, error) {
	return m.validateToken(tokenString, m.secretKey)
}

func (m *JWTManager) validateToken(tokenString string, secretKey string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, err)
	}

	if claims, ok := token.Claims.(*AuthClaims); ok && token.Valid {
		if claims.Issuer != m.issuer {
			return nil, fmt.Errorf("%w: invalid issuer", ErrTokenInvalid)
		}
		if claims.PhoneNumber == "" {
			return nil, ErrMissingClaims
		}
		return claims, nil
	}

	return nil, ErrTokenInvalid
}
