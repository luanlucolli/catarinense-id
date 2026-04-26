package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	Requests        int
	Window          time.Duration
	Burst           int
	CleanupInterval time.Duration
	InactiveTTL     time.Duration
	KeyFunc         func(*gin.Context) string
}

type rateLimitedClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu      sync.Mutex
	clients map[string]*rateLimitedClient
	config  RateLimitConfig
}

func NewRateLimiter(config RateLimitConfig) gin.HandlerFunc {
	config = normalizeRateLimitConfig(config)

	store := &rateLimiterStore{
		clients: make(map[string]*rateLimitedClient),
		config:  config,
	}

	go store.cleanupLoop()

	return func(c *gin.Context) {
		key := strings.TrimSpace(config.KeyFunc(c))
		if key == "" {
			key = "unknown"
		}

		limiter := store.limiterFor(key)
		if !limiter.Allow() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "limite de requisições excedido",
			})
			return
		}

		c.Next()
	}
}

func normalizeRateLimitConfig(config RateLimitConfig) RateLimitConfig {
	if config.Requests <= 0 {
		config.Requests = 120
	}

	if config.Window <= 0 {
		config.Window = time.Minute
	}

	if config.Burst <= 0 {
		config.Burst = min(config.Requests, 20)
	}

	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	if config.InactiveTTL <= 0 {
		config.InactiveTTL = 10 * time.Minute
	}

	if config.KeyFunc == nil {
		config.KeyFunc = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}

	return config
}

func (s *rateLimiterStore) limiterFor(key string) *rate.Limiter {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[key]
	if !exists {
		client = &rateLimitedClient{
			limiter: rate.NewLimiter(rate.Limit(float64(s.config.Requests)/s.config.Window.Seconds()), s.config.Burst),
		}
		s.clients[key] = client
	}

	client.lastSeen = now
	return client.limiter
}

func (s *rateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-s.config.InactiveTTL)

		s.mu.Lock()
		for key, client := range s.clients {
			if client.lastSeen.Before(cutoff) {
				delete(s.clients, key)
			}
		}
		s.mu.Unlock()
	}
}
