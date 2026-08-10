package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidToken     = errors.New("invalid token")
	ErrSessionExpired   = errors.New("session expired")
	ErrUsernameTaken    = errors.New("username already taken")
	ErrSSAConfigMissing = errors.New("SSA not configured")
)

type AuthService struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
	cacheRepo   CacheRepository
	ssaClient   SSAClient
	jwtConfig   *auth.Config
	indexer     Indexer
}

// UserRepository — интерфейс пользователей для сервисного слоя.
type UserRepository interface {
	Create(user *models.User) error
	FindBySSAID(ssaID string) (*models.User, error)
	FindByID(id string) (*models.User, error)
	Update(user *models.User) error
	UpdateLastSeen(userID string) error
	UpdateUsername(userID, username string) error
	CheckUsername(username string) (bool, error)
}

// SessionRepository — интерфейс сессий для сервисного слоя.
type SessionRepository interface {
	Create(session *models.Session) error
	FindByRefreshToken(token string) (*models.Session, error)
	Revoke(id string) error
	RevokeByUserID(userID string) error
}

// CacheRepository — интерфейс кеша для сервисного слоя.
type CacheRepository interface {
	SetSession(ctx context.Context, key string, session *models.Session, ttl time.Duration) error
	GetSession(ctx context.Context, key string) (*models.Session, error)
	DelSession(ctx context.Context, key string) error
	SetUser(ctx context.Context, userID string, user *models.User) error
	GetUser(ctx context.Context, userID string) (*models.User, error)
	DelUser(ctx context.Context, userID string) error
}

// SSAClient — интерфейс OAuth-провайдера для сервисного слоя.
type SSAClient interface {
	AuthorizeURL(state string) string
	ExchangeCode(code string) (*repository.SSATokenResponse, error)
	GetUserInfo(accessToken string) (*repository.SSAUserInfo, error)
}

func NewAuthService(
	userRepo UserRepository,
	sessionRepo SessionRepository,
	cacheRepo CacheRepository,
	ssaClient SSAClient,
	jwtConfig *auth.Config,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		cacheRepo:   cacheRepo,
		ssaClient:   ssaClient,
		jwtConfig:   jwtConfig,
	}
}

func (s *AuthService) SetIndexer(i Indexer) {
	s.indexer = i
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *AuthService) GetSSAAuthorizeURL(state string) string {
	return s.ssaClient.AuthorizeURL(state)
}

func (s *AuthService) HandleSSACallback(ctx context.Context, code string, userAgent string, ip string) (*TokenPair, *models.User, error) {
	tokenResp, err := s.ssaClient.ExchangeCode(code)
	if err != nil {
		return nil, nil, fmt.Errorf("SSA code exchange failed: %w", err)
	}

	ssaUser, err := s.ssaClient.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("SSA userinfo failed: %w", err)
	}

	user, err := s.userRepo.FindBySSAID(ssaUser.Sub)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("failed to find user: %w", err)
		}

		user = &models.User{
			SSAID:       ssaUser.Sub,
			Email:       ssaUser.Email,
			DisplayName: ssaUser.Name,
			Phone:       ssaUser.Phone,
			Role:        "user",
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	if err := s.userRepo.UpdateLastSeen(user.ID); err != nil {
		return nil, nil, fmt.Errorf("failed to update last seen: %w", err)
	}

	tokens, err := s.createSession(ctx, user, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}

	s.cacheRepo.SetUser(ctx, user.ID, user)

	return tokens, user, nil
}

func (s *AuthService) Register(ctx context.Context, ssaCode string, username string, displayName string, userAgent string, ip string) (*TokenPair, *models.User, error) {
	if username != "" {
		exists, err := s.userRepo.CheckUsername(username)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to check username: %w", err)
		}
		if exists {
			return nil, nil, ErrUsernameTaken
		}
	}

	tokenResp, err := s.ssaClient.ExchangeCode(ssaCode)
	if err != nil {
		return nil, nil, fmt.Errorf("SSA code exchange failed: %w", err)
	}

	ssaUser, err := s.ssaClient.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("SSA userinfo failed: %w", err)
	}

	existing, err := s.userRepo.FindBySSAID(ssaUser.Sub)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to find user: %w", err)
	}
	if existing != nil {
		tokens, err := s.createSession(ctx, existing, userAgent, ip)
		if err != nil {
			return nil, nil, err
		}
		return tokens, existing, nil
	}

	user := &models.User{
		SSAID:       ssaUser.Sub,
		Email:       ssaUser.Email,
		DisplayName: displayName,
		Phone:       ssaUser.Phone,
		Username:    username,
		Role:        "user",
	}
	if user.DisplayName == "" {
		user.DisplayName = ssaUser.Name
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	if s.indexer != nil {
		go s.indexer.IndexUser(user)
	}

	tokens, err := s.createSession(ctx, user, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}

	return tokens, user, nil
}

func (s *AuthService) Login(ctx context.Context, ssaCode string, userAgent string, ip string) (*TokenPair, *models.User, error) {
	return s.HandleSSACallback(ctx, ssaCode, userAgent, ip)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	cached, err := s.cacheRepo.GetSession(ctx, refreshToken)
	if err == nil && cached != nil {
		if cached.IsRevoked || time.Now().After(cached.ExpiresAt) {
			s.sessionRepo.Revoke(cached.ID)
			s.cacheRepo.DelSession(ctx, refreshToken)
			return nil, ErrSessionExpired
		}
		user, err := s.userRepo.FindByID(cached.UserID)
		if err != nil {
			return nil, ErrUserNotFound
		}
		return s.generateTokenPair(ctx, user, cached.UserAgent, cached.IP)
	}

	session, err := s.sessionRepo.FindByRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to find session: %w", err)
	}

	if session.IsRevoked || time.Now().After(session.ExpiresAt) {
		s.sessionRepo.Revoke(session.ID)
		return nil, ErrSessionExpired
	}

	if err := s.sessionRepo.Revoke(session.ID); err != nil {
		return nil, fmt.Errorf("failed to revoke old session: %w", err)
	}

	user, err := s.userRepo.FindByID(session.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return s.generateTokenPair(ctx, user, session.UserAgent, session.IP)
}

func (s *AuthService) Logout(ctx context.Context, userID string, refreshToken string) error {
	if refreshToken != "" {
		s.cacheRepo.DelSession(ctx, refreshToken)
		session, err := s.sessionRepo.FindByRefreshToken(refreshToken)
		if err == nil {
			s.sessionRepo.Revoke(session.ID)
		}
	}
	s.cacheRepo.DelUser(ctx, userID)
	return s.sessionRepo.RevokeByUserID(userID)
}

func (s *AuthService) GetMe(ctx context.Context, userID string) (*models.User, error) {
	cached, err := s.cacheRepo.GetUser(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	s.cacheRepo.SetUser(ctx, userID, user)
	return user, nil
}

func (s *AuthService) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*models.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if v, ok := updates["display_name"]; ok {
		user.DisplayName = v.(string)
	}
	if v, ok := updates["bio"]; ok {
		user.Bio = v.(string)
	}
	if v, ok := updates["avatar_url"]; ok {
		user.AvatarURL = v.(string)
	}
	if v, ok := updates["status_text"]; ok {
		user.StatusText = v.(string)
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.cacheRepo.DelUser(ctx, userID)
	return user, nil
}

func (s *AuthService) CheckUsername(username string) (bool, error) {
	return s.userRepo.CheckUsername(username)
}

func (s *AuthService) UpdateUsername(ctx context.Context, userID string, username string) error {
	exists, err := s.userRepo.CheckUsername(username)
	if err != nil {
		return fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return ErrUsernameTaken
	}

	if err := s.userRepo.UpdateUsername(userID, username); err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	s.cacheRepo.DelUser(ctx, userID)
	return nil
}

func (s *AuthService) createSession(ctx context.Context, user *models.User, userAgent string, ip string) (*TokenPair, error) {
	return s.generateTokenPair(ctx, user, userAgent, ip)
}

func (s *AuthService) generateTokenPair(ctx context.Context, user *models.User, userAgent string, ip string) (*TokenPair, error) {
	accessToken, err := auth.GenerateAccessToken(s.jwtConfig, user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := auth.GenerateRefreshToken(s.jwtConfig, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	session := &models.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		ExpiresAt:    time.Now().Add(s.jwtConfig.RefreshTTL),
	}

	if err := s.sessionRepo.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.cacheRepo.SetSession(ctx, refreshToken, session, s.jwtConfig.RefreshTTL)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.jwtConfig.AccessTTL.Seconds()),
	}, nil
}

func GenerateState() string {
	return uuid.New().String()
}
