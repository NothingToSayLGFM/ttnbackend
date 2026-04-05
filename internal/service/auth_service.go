package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type AuthService struct {
	users     *repository.UserRepo
	tokens    *repository.TokenRepo
	jwtSecret []byte
}

func NewAuthService(users *repository.UserRepo, tokens *repository.TokenRepo, jwtSecret string) *AuthService {
	return &AuthService{users: users, tokens: tokens, jwtSecret: []byte(jwtSecret)}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *AuthService) Register(ctx context.Context, email, name, password string) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u := &domain.User{
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, domain.ErrConflict
	}
	created, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, *TokenPair, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, domain.ErrUnauthorized
	}
	pair, err := s.issueTokens(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userID, err := s.tokens.FindUserID(ctx, refreshToken)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	if err := s.tokens.Revoke(ctx, refreshToken); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, userID)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.tokens.Revoke(ctx, refreshToken)
}

func (s *AuthService) ValidateAccessToken(tokenStr string) (string, domain.Role, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", "", domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", domain.ErrUnauthorized
	}
	userID, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	return userID, domain.Role(role), nil
}

func (s *AuthService) issueTokens(ctx context.Context, userID string) (*TokenPair, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	expiresIn := 15 * 60
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": string(u.Role),
		"exp":  time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := generateToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := s.tokens.Save(ctx, userID, refreshToken, expiresAt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
