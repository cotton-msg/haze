package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	cfg := &Config{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	token, err := GenerateAccessToken(cfg, "user-1", "testuser", "user")
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("expected testuser, got %s", claims.Username)
	}
	if claims.Role != "user" {
		t.Errorf("expected user, got %s", claims.Role)
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	cfg := &Config{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	token, err := GenerateRefreshToken(cfg, "user-1")
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	if token == "" {
		t.Fatal("refresh token should not be empty")
	}
}

func TestInvalidToken(t *testing.T) {
	cfg := &Config{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	_, err := ValidateAccessToken(cfg, "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestWrongSecret(t *testing.T) {
	cfg := &Config{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	token, err := GenerateAccessToken(cfg, "user-1", "testuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	wrongCfg := &Config{AccessSecret: "wrong-secret"}
	_, err = ValidateAccessToken(wrongCfg, token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

// TestRejectsNonHS256Algorithm защищает от алгоритм-конфьюшн атак:
// токен, подписанный другим алгоритмом с тем же секретом, не должен проходить.
func TestRejectsNonHS256Algorithm(t *testing.T) {
	cfg := &Config{AccessSecret: "test-secret"}

	claims := Claims{
		UserID:   "user-1",
		Username: "testuser",
		Role:     "user",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(cfg.AccessSecret))
	if err != nil {
		t.Fatalf("failed to sign HS512 token: %v", err)
	}

	_, err = ValidateAccessToken(cfg, signed)
	if err == nil {
		t.Fatal("expected error for non-HS256 algorithm")
	}
}
