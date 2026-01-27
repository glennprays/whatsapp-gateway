package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type SwaggerSession struct {
	sessions map[string]time.Time
	mu       sync.RWMutex
	username string
	password string
}

func NewSwaggerSessionAuth(username, password string) *SwaggerSession {
	s := &SwaggerSession{
		sessions: make(map[string]time.Time),
		username: username,
		password: password,
	}
	go s.cleanupExpiredSessions()
	return s
}

func (s *SwaggerSession) getRedirectPath(c *fiber.Ctx, basePath string) string {
	redirect := c.Query("redirect")
	if redirect == "" {
		return ""
	}

	if !strings.HasPrefix(redirect, basePath) {
		return ""
	}

	if strings.Contains(redirect, "://") {
		return ""
	}

	return redirect
}

func (s *SwaggerSession) Handler(basePath string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if there's a valid session cookie
		sessionToken := c.Cookies("swagger_session")
		if sessionToken != "" && s.isValidSession(sessionToken) {
			return c.Next()
		}

		loginPath := basePath + "/login"
		currentPath := c.Path()

		if c.Method() == "POST" && currentPath == loginPath {
			return s.handleLogin(c, basePath)
		}

		if currentPath == loginPath {
			return c.SendFile("docs/swagger-ui/swagger-login.html")
		}

		redirectURL := loginPath + "?redirect=" + url.QueryEscape(currentPath)
		return c.Redirect(redirectURL)
	}
}

func (s *SwaggerSession) handleLogin(c *fiber.Ctx, basePath string) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	if username == s.username && password == s.password {
		// Create session
		token := s.createSession()

		// Set cookie
		c.Cookie(&fiber.Cookie{
			Name:     "swagger_session",
			Value:    token,
			Expires:  time.Now().Add(24 * time.Hour),
			HTTPOnly: true,
			SameSite: "Lax",
		})

		// Get redirect path from query parameter
		redirect := s.getRedirectPath(c, basePath)
		if redirect == "" {
			redirect = basePath
		}

		return c.Redirect(redirect)
	}

	// Failed login - redirect back to login page with error parameter
	redirect := s.getRedirectPath(c, basePath)
	loginURL := basePath + "/login?error=1"
	if redirect != "" {
		loginURL += "&redirect=" + url.QueryEscape(redirect)
	}
	return c.Redirect(loginURL)
}

func (s *SwaggerSession) createSession() string {
	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)

	s.mu.Lock()
	s.sessions[tokenStr] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()

	return tokenStr
}

func (s *SwaggerSession) isValidSession(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expiry, exists := s.sessions[token]
	if !exists {
		return false
	}

	return time.Now().Before(expiry)
}

func (s *SwaggerSession) cleanupExpiredSessions() {
	ticker := time.NewTicker(30 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for token, expiry := range s.sessions {
			if now.After(expiry) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}
