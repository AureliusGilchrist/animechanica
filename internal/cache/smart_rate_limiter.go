package cache

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SmartRateLimiter provides intelligent rate limiting for API calls
type SmartRateLimiter struct {
	logger *zerolog.Logger
	mu     sync.RWMutex

	// Rate limiting configuration
	requestsPerMinute int
	burstSize         int

	// Request tracking
	requestTimes []time.Time
	lastRequest  time.Time

	// Adaptive rate limiting
	consecutiveErrors int
	backoffMultiplier float64
	maxBackoffDelay   time.Duration

	// Request priority queues
	highPriorityQueue   []func()
	normalPriorityQueue []func()
	lowPriorityQueue    []func()

	// Processing state
	isProcessing bool
}

func NewSmartRateLimiter(logger *zerolog.Logger) *SmartRateLimiter {
	return &SmartRateLimiter{
		logger: logger,

		requestsPerMinute: 90,  // AniList allows 90 requests per minute
		burstSize:         10,  // Allow burst of 10 requests
		
		requestTimes:      make([]time.Time, 0),
		backoffMultiplier: 1.0,
		maxBackoffDelay:   30 * time.Second,

		highPriorityQueue:   make([]func(), 0),
		normalPriorityQueue: make([]func(), 0),
		lowPriorityQueue:    make([]func(), 0),
	}
}

// Priority levels for API requests
type RequestPriority int

const (
	HighPriority   RequestPriority = iota // User-initiated requests
	NormalPriority                        // Background updates
	LowPriority                           // Prefetching, non-critical
)

// ExecuteWithRateLimit executes a function with intelligent rate limiting
func (srl *SmartRateLimiter) ExecuteWithRateLimit(ctx context.Context, priority RequestPriority, fn func() error) error {
	srl.mu.Lock()
	defer srl.mu.Unlock()

	// Add to appropriate priority queue
	resultChan := make(chan error, 1)
	wrappedFn := func() {
		err := fn()
		if err != nil {
			srl.handleError()
		} else {
			srl.handleSuccess()
		}
		resultChan <- err
	}

	switch priority {
	case HighPriority:
		srl.highPriorityQueue = append(srl.highPriorityQueue, wrappedFn)
	case NormalPriority:
		srl.normalPriorityQueue = append(srl.normalPriorityQueue, wrappedFn)
	case LowPriority:
		srl.lowPriorityQueue = append(srl.lowPriorityQueue, wrappedFn)
	}

	// Start processing if not already running
	if !srl.isProcessing {
		go srl.processQueue()
	}

	srl.mu.Unlock()
	
	// Wait for result
	select {
	case err := <-resultChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// processQueue processes requests from priority queues with rate limiting
func (srl *SmartRateLimiter) processQueue() {
	srl.mu.Lock()
	srl.isProcessing = true
	srl.mu.Unlock()

	defer func() {
		srl.mu.Lock()
		srl.isProcessing = false
		srl.mu.Unlock()
	}()

	for {
		srl.mu.Lock()
		
		// Get next function to execute based on priority
		var nextFn func()
		var queueName string

		if len(srl.highPriorityQueue) > 0 {
			nextFn = srl.highPriorityQueue[0]
			srl.highPriorityQueue = srl.highPriorityQueue[1:]
			queueName = "high"
		} else if len(srl.normalPriorityQueue) > 0 {
			nextFn = srl.normalPriorityQueue[0]
			srl.normalPriorityQueue = srl.normalPriorityQueue[1:]
			queueName = "normal"
		} else if len(srl.lowPriorityQueue) > 0 {
			nextFn = srl.lowPriorityQueue[0]
			srl.lowPriorityQueue = srl.lowPriorityQueue[1:]
			queueName = "low"
		}

		if nextFn == nil {
			srl.mu.Unlock()
			return // No more requests to process
		}

		// Check if we need to wait due to rate limiting
		delay := srl.calculateDelay()
		srl.mu.Unlock()

		if delay > 0 {
			srl.logger.Debug().
				Dur("delay", delay).
				Str("queue", queueName).
				Msg("Smart rate limiter: Delaying request")
			time.Sleep(delay)
		}

		// Execute the function
		srl.recordRequest()
		nextFn()
	}
}

// calculateDelay calculates how long to wait before making the next request
func (srl *SmartRateLimiter) calculateDelay() time.Duration {
	now := time.Now()

	// Clean old request times (older than 1 minute)
	cutoff := now.Add(-time.Minute)
	validTimes := make([]time.Time, 0)
	for _, t := range srl.requestTimes {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}
	srl.requestTimes = validTimes

	// Check if we're within rate limits
	if len(srl.requestTimes) < srl.requestsPerMinute {
		// Check burst limit
		if len(srl.requestTimes) < srl.burstSize {
			return 0 // No delay needed
		}

		// Calculate delay to maintain smooth rate
		if len(srl.requestTimes) > 0 {
			timeSinceLastRequest := now.Sub(srl.lastRequest)
			minInterval := time.Minute / time.Duration(srl.requestsPerMinute)
			
			if timeSinceLastRequest < minInterval {
				return minInterval - timeSinceLastRequest
			}
		}
		return 0
	}

	// We're at the rate limit, calculate when we can make the next request
	oldestRequest := srl.requestTimes[0]
	return time.Minute - now.Sub(oldestRequest)
}

// recordRequest records a new request timestamp
func (srl *SmartRateLimiter) recordRequest() {
	srl.mu.Lock()
	defer srl.mu.Unlock()

	now := time.Now()
	srl.requestTimes = append(srl.requestTimes, now)
	srl.lastRequest = now

	srl.logger.Debug().
		Int("totalRequests", len(srl.requestTimes)).
		Float64("backoffMultiplier", srl.backoffMultiplier).
		Msg("Smart rate limiter: Request recorded")
}

// handleError handles API request errors and adjusts rate limiting
func (srl *SmartRateLimiter) handleError() {
	srl.consecutiveErrors++
	
	// Increase backoff multiplier for consecutive errors
	if srl.consecutiveErrors > 3 {
			if srl.backoffMultiplier * 1.5 < 5.0 {
				srl.backoffMultiplier = srl.backoffMultiplier * 1.5
			} else {
				srl.backoffMultiplier = 5.0
			}
		srl.logger.Warn().
			Int("consecutiveErrors", srl.consecutiveErrors).
			Float64("newBackoffMultiplier", srl.backoffMultiplier).
			Msg("Smart rate limiter: Increasing backoff due to errors")
	}
}

// handleSuccess handles successful API requests
func (srl *SmartRateLimiter) handleSuccess() {
	if srl.consecutiveErrors > 0 {
		srl.consecutiveErrors = 0
		srl.backoffMultiplier = max(srl.backoffMultiplier*0.8, 1.0)
		srl.logger.Debug().
			Float64("newBackoffMultiplier", srl.backoffMultiplier).
			Msg("Smart rate limiter: Reducing backoff after success")
	}
}

// GetQueueStats returns statistics about the rate limiter queues
func (srl *SmartRateLimiter) GetQueueStats() map[string]interface{} {
	srl.mu.RLock()
	defer srl.mu.RUnlock()

	return map[string]interface{}{
		"queue_lengths": map[string]int{
			"high_priority":   len(srl.highPriorityQueue),
			"normal_priority": len(srl.normalPriorityQueue),
			"low_priority":    len(srl.lowPriorityQueue),
		},
		"rate_limiting": map[string]interface{}{
			"requests_per_minute": srl.requestsPerMinute,
			"burst_size":          srl.burstSize,
			"recent_requests":     len(srl.requestTimes),
			"consecutive_errors":  srl.consecutiveErrors,
			"backoff_multiplier":  srl.backoffMultiplier,
		},
		"status": map[string]interface{}{
			"is_processing": srl.isProcessing,
			"last_request":  srl.lastRequest,
		},
	}
}

// Helper functions for min/max
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
