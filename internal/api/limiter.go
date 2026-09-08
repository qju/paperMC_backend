package api

import (
	"sync"
	"time"
)

type ipAttempt struct {
	count     int
	lastFail  time.Time
	blockedTo time.Time
}

// LoginRateLimiter enforces brute-force protection per client IP address.
type LoginRateLimiter struct {
	mu           sync.Mutex
	attempts     map[string]*ipAttempt
	maxAttempts  int
	window       time.Duration
	lockDuration time.Duration
}

// NewLoginRateLimiter creates a new thread-safe rate limiter.
func NewLoginRateLimiter(maxAttempts int, window, lockDuration time.Duration) *LoginRateLimiter {
	limiter := &LoginRateLimiter{
		attempts:     make(map[string]*ipAttempt),
		maxAttempts:  maxAttempts,
		window:       window,
		lockDuration: lockDuration,
	}
	go limiter.cleanupLoop()
	return limiter
}

// Allow returns true if the IP is currently permitted to attempt login.
func (l *LoginRateLimiter) Allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, exists := l.attempts[ip]
	if !exists {
		return true, 0
	}

	if now.Before(entry.blockedTo) {
		return false, entry.blockedTo.Sub(now)
	}

	if now.Sub(entry.lastFail) > l.window {
		delete(l.attempts, ip)
		return true, 0
	}

	return true, 0
}

// RecordFailure records an authentication failure for the IP.
func (l *LoginRateLimiter) RecordFailure(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, exists := l.attempts[ip]
	if !exists || now.Sub(entry.lastFail) > l.window {
		entry = &ipAttempt{count: 1, lastFail: now}
		l.attempts[ip] = entry
		return false, 0
	}

	entry.count++
	entry.lastFail = now

	if entry.count >= l.maxAttempts {
		entry.blockedTo = now.Add(l.lockDuration)
		return true, l.lockDuration
	}

	return false, 0
}

// Reset clears failure tracking for the specified IP upon successful login.
func (l *LoginRateLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func (l *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, entry := range l.attempts {
			if now.After(entry.blockedTo) && now.Sub(entry.lastFail) > l.window {
				delete(l.attempts, ip)
			}
		}
		l.mu.Unlock()
	}
}
