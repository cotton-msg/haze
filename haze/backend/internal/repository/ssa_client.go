package repository

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SSAConfig struct {
	ClientID     string
	ClientSecret string
	AuthorizeURL string
	TokenURL     string
	RedirectURL  string
}

type SSATokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

type SSAUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Phone string `json:"phone_number,omitempty"`
}

type SSAClient struct {
	config     *SSAConfig
	httpClient *http.Client
}

func NewSSAClient(config *SSAConfig) *SSAClient {
	return &SSAClient{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *SSAClient) AuthorizeURL(state string) string {
	v := url.Values{}
	v.Set("client_id", c.config.ClientID)
	v.Set("redirect_uri", c.config.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", "openid profile email")
	v.Set("state", state)
	return c.config.AuthorizeURL + "?" + v.Encode()
}

func (c *SSAClient) ExchangeCode(code string) (*SSATokenResponse, error) {
	v := url.Values{}
	v.Set("grant_type", "authorization_code")
	v.Set("code", code)
	v.Set("redirect_uri", c.config.RedirectURL)
	v.Set("client_id", c.config.ClientID)
	v.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequest("POST", c.config.TokenURL, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SSA token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp SSATokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

func (c *SSAClient) GetUserInfo(accessToken string) (*SSAUserInfo, error) {
	req, err := http.NewRequest("GET", c.config.AuthorizeURL+"/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var userInfo SSAUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}
