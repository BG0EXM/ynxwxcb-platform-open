package middleware

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	mu   sync.Mutex
	hits []time.Time
}

// rateLimiter 简单的滑动窗口计数器（按 key 限流）
type rateLimiter struct {
	visitors sync.Map
	max      int
	window   time.Duration
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	vIface, _ := l.visitors.LoadOrStore(key, &visitor{})
	v := vIface.(*visitor)

	v.mu.Lock()
	defer v.mu.Unlock()

	var kept []time.Time
	for _, t := range v.hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.max {
		v.hits = kept
		return false
	}
	
	v.hits = append(kept, now)
	return true
}

func (l *rateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		now := time.Now()
		cutoff := now.Add(-l.window)
		
		l.visitors.Range(func(key, value interface{}) bool {
			v := value.(*visitor)
			v.mu.Lock()
			if len(v.hits) == 0 || !v.hits[len(v.hits)-1].After(cutoff) {
				l.visitors.Delete(key)
			}
			v.mu.Unlock()
			return true
		})
	}
}

// RateLimit 按客户端 IP 限流：window 时间内最多 max 次
func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	l := &rateLimiter{max: max, window: window}
	go l.cleanup()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(ClientIP(r)) {
				JSON(w, http.StatusTooManyRequests, map[string]string{"error": "操作过于频繁，请稍后再试"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
