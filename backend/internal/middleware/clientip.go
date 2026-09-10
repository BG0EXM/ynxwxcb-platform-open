package middleware

import (
	"net"
	"net/http"
	"strings"
)

// trustProxy 是否信任 X-Forwarded-For（前置反向代理/WAF 时由配置开启）
var trustProxy bool

// SetTrustProxy 设置是否信任 X-Forwarded-For
func SetTrustProxy(v bool) { trustProxy = v }

// ClientIP 提取客户端 IP：仅当信任代理时用 X-Forwarded-For，否则用连接地址；兼容 IPv4/IPv6
func ClientIP(r *http.Request) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
