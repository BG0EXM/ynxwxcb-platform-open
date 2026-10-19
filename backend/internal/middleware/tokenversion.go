package middleware

import (
	"database/sql"
	"sync"

	"ynxwxcb-platform/internal/database"
)

// sessionUser 认证所需的用户实时状态（以数据库为准，不信任令牌中的角色与姓名）
type sessionUser struct {
	TokenVersion int
	Status       int
	Username     string
	RealName     string
	RoleCode     string
	MustChange   bool
}

// 用户状态缓存：userID -> sessionUser；gen 用于防止“读库期间被失效、随后又把旧值写回缓存”的竞态
var (
	tvMu    sync.RWMutex
	tvCache = map[int64]sessionUser{}
	tvGen   = map[int64]uint64{}
)

// loadSessionUser 读取用户当前令牌版本、状态、角色（带内存缓存）
func loadSessionUser(userID int64) (sessionUser, bool) {
	tvMu.RLock()
	u, ok := tvCache[userID]
	gen := tvGen[userID]
	tvMu.RUnlock()
	if ok {
		return u, true
	}
	var role, realName sql.NullString
	var mustChange sql.NullInt64
	err := database.DB.QueryRow(
		`SELECT u.token_version, u.status, u.username, u.real_name, r.code, u.must_change_password
		 FROM users u LEFT JOIN roles r ON u.role_id = r.id WHERE u.id = ?`, userID).
		Scan(&u.TokenVersion, &u.Status, &u.Username, &realName, &role, &mustChange)
	if err != nil {
		return sessionUser{}, false
	}
	u.RealName = realName.String
	u.RoleCode = role.String
	u.MustChange = mustChange.Int64 == 1
	tvMu.Lock()
	if tvGen[userID] == gen {
		tvCache[userID] = u
	}
	tvMu.Unlock()
	return u, true
}

// currentTokenVersion 读取用户当前令牌版本（兼容旧调用）
func currentTokenVersion(userID int64) (int, bool) {
	u, ok := loadSessionUser(userID)
	return u.TokenVersion, ok
}

// InvalidateTokenVersion 清除某用户状态缓存（改密/禁用/改角色/登出/删除后调用，下次请求重新读取）
func InvalidateTokenVersion(userID int64) {
	tvMu.Lock()
	delete(tvCache, userID)
	tvGen[userID]++
	tvMu.Unlock()
}
