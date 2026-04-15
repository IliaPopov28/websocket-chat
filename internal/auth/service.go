// GRACE: auth — регистрация, логин, JWT-токены.
//
// ПОТОК:
//   - Register() → INSERT в БД с bcrypt hash → если 23505 (unique violation) → ErrUserAlreadyExists
//   - Login() → SELECT по nickname → bcrypt.CompareHashAndPassword → jwt.Sign
//   - ValidateToken() → jwt.Parse с проверкой подписи и expiration
//
// DECISION: JWT без refresh — упрощение для pet-проекта. Токен на 24 часа.
// В production нужен refresh endpoint.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/IliaPopov28/websocket-chat/internal/store/postgres"
)

// Re-export ошибок из postgres для удобства.
var (
	ErrUserNotFound      = postgres.ErrUserNotFound
	ErrUserAlreadyExists = postgres.ErrUserAlreadyExists
)

type Service struct {
	store  *postgres.UserStore
	secret []byte
}

func NewService(store *postgres.UserStore, secret string) *Service {
	return &Service{
		store:  store,
		secret: []byte(secret),
	}
}

// Register создаёт нового пользователя с хэшированным паролем.
func (s *Service) Register(ctx context.Context, nickname, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.store.Create(ctx, nickname, string(hash))
}

// Login проверяет пароль и возвращает JWT-токен.
func (s *Service) Login(ctx context.Context, nickname, password string) (string, error) {
	user, err := s.store.GetByNickname(ctx, nickname)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid password")
	}

	return s.generateToken(user.Nickname)
}

// ValidateToken проверяет JWT и возвращает nickname.
func (s *Service) ValidateToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	nickname, ok := claims["nickname"].(string)
	if !ok {
		return "", fmt.Errorf("missing nickname in token")
	}

	return nickname, nil
}

func (s *Service) generateToken(nickname string) (string, error) {
	claims := jwt.MapClaims{
		"nickname": nickname,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}
