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

type DocumentationSession struct {
	sessions map[string]time.Time
	mu       sync.RWMutex
	username string
	password string
}

func NewDocsSessionAuth(username, password string) *DocumentationSession {
	s := &DocumentationSession{
		sessions: make(map[string]time.Time),
		username: username,
		password: password,
	}
	go s.cleanupExpiredSessions()
	return s
}

func (s *DocumentationSession) getRedirectPath(c *fiber.Ctx, basePath string) string {
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

func (s *DocumentationSession) Handler(basePath string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if there's a valid session cookie
		sessionToken := c.Cookies("docs_session")
		if sessionToken != "" && s.isValidSession(sessionToken) {
			return c.Next()
		}

		loginPath := basePath + "/login"
		currentPath := c.Path()

		if c.Method() == "POST" && currentPath == loginPath {
			return s.handleLogin(c, basePath)
		}

		if currentPath == loginPath {
			return c.SendFile("docs/ui/login.html")
		}

		redirectURL := loginPath + "?redirect=" + url.QueryEscape(currentPath)
		return c.Redirect(redirectURL)
	}
}

func (s *DocumentationSession) handleLogin(c *fiber.Ctx, basePath string) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	if username == s.username && password == s.password {
		// Create session
		token := s.createSession()

		// Set cookie
		c.Cookie(&fiber.Cookie{
			Name:     "docs_session",
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

func (s *DocumentationSession) createSession() string {
	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)

	s.mu.Lock()
	s.sessions[tokenStr] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()

	return tokenStr
}

func (s *DocumentationSession) isValidSession(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expiry, exists := s.sessions[token]
	if !exists {
		return false
	}

	return time.Now().Before(expiry)
}

func (s *DocumentationSession) cleanupExpiredSessions() {
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
