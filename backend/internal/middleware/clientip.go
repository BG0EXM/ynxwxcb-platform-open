package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// trustProxy 是否信任反向代理传递的客户端 IP（前置 nginx/Caddy/WAF 时由配置开启）
var trustProxy bool

// SetTrustProxy 设置是否信任反向代理传递的客户端 IP
func SetTrustProxy(v bool) { trustProxy = v }

// ClientIP 提取客户端 IP，兼容 IPv4/IPv6：
//   - 仅当开启 trust_proxy 且连接直接来自本机或内网代理时，才采信代理头，防止外网直连伪造；
//   - 优先 X-Real-IP（nginx 以 $remote_addr 覆盖设置，客户端无法伪造）；
//   - 其次取 X-Forwarded-For 最右一段（最近一级代理追加的真实来源），最左段可被客户端任意伪造，绝不采信。
func ClientIP(r *http.Request) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if trustProxy && isTrustedProxy(remote) {
		if ip := parseIP(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := parseIP(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	return remote
}

// isTrustedProxy 本机回环或内网地址视为可信代理
func isTrustedProxy(addr string) bool {
	ip, err := netip.ParseAddr(strings.TrimSpace(addr))
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate()
}

func parseIP(s string) string {
	ip, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return ""
	}
	return ip.Unmap().String()
}
