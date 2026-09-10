package middleware

import (
	"sync"

	"ynxwxcb-platform/internal/database"
)

// 用户令牌版本缓存：userID -> token_version
var (
	tvMu    sync.RWMutex
	tvCache = map[int64]int{}
)

// currentTokenVersion 读取用户当前令牌版本（带内存缓存）
func currentTokenVersion(userID int64) (int, bool) {
	tvMu.RLock()
	v, ok := tvCache[userID]
	tvMu.RUnlock()
	if ok {
		return v, true
	}
	var ver int
	if err := database.DB.QueryRow("SELECT token_version FROM users WHERE id = ?", userID).Scan(&ver); err != nil {
		return 0, false
	}
	tvMu.Lock()
	tvCache[userID] = ver
	tvMu.Unlock()
	return ver, true
}

// InvalidateTokenVersion 清除某用户令牌版本缓存（改密/禁用/登出后调用，下次请求重新读取）
func InvalidateTokenVersion(userID int64) {
	tvMu.Lock()
	delete(tvCache, userID)
	tvMu.Unlock()
}
