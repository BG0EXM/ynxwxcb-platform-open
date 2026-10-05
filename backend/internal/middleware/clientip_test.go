package middleware

import (
	"net/http"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		trustProxy bool
		xff        string
		remoteAddr string
		expected   string
	}{
		{
			name:       "trust proxy, with X-Forwarded-For",
			trustProxy: true,
			xff:        "192.168.1.1, 10.0.0.1",
			remoteAddr: "127.0.0.1:8080",
			expected:   "192.168.1.1",
		},
		{
			name:       "trust proxy, empty X-Forwarded-For",
			trustProxy: true,
			xff:        "",
			remoteAddr: "127.0.0.1:8080",
			expected:   "127.0.0.1",
		},
		{
			name:       "no trust proxy, with X-Forwarded-For",
			trustProxy: false,
			xff:        "192.168.1.1, 10.0.0.1",
			remoteAddr: "10.0.0.5:8080",
			expected:   "10.0.0.5",
		},
		{
			name:       "no trust proxy, no port in RemoteAddr",
			trustProxy: false,
			xff:        "",
			remoteAddr: "10.0.0.6",
			expected:   "10.0.0.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTrustProxy(tt.trustProxy)
			
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			req.RemoteAddr = tt.remoteAddr
			
			got := ClientIP(req)
			if got != tt.expected {
				t.Errorf("ClientIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}
