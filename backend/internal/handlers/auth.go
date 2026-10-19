package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// 登录限流：同一 IP 对同一账号最多连续失败 5 次，锁定 10 分钟。
// 按“用户名+IP”计数：攻击者只能锁住自己那台机器，无法远程把 admin 等账号锁死；
// 另有路由层按 IP 每分钟 20 次的总限流兜底。
const (
	maxLoginFails = 5
	lockDuration  = 10 * time.Minute
	failTTL       = 30 * time.Minute // 失败计数在无新失败 30 分钟后自动过期
)

type loginFailEntry struct {
	fails     int
	lockUntil time.Time
	lastFail  time.Time
}

type loginFailCounter struct {
	mu        sync.Mutex
	entries   map[string]*loginFailEntry
	lastSweep time.Time
}

var loginLimiter = &loginFailCounter{entries: make(map[string]*loginFailEntry)}

func loginKey(username, ip string) string { return username + "|" + ip }

// sweepLocked 定期清理过期条目，防止内存只增不减（调用方已持锁）
func (l *loginFailCounter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for k, e := range l.entries {
		if now.After(e.lockUntil) && now.Sub(e.lastFail) > failTTL {
			delete(l.entries, k)
		}
	}
}

// checkLocked 检查是否被锁定
func (l *loginFailCounter) checkLocked(username, ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweepLocked(now)
	e, ok := l.entries[loginKey(username, ip)]
	if !ok {
		return false
	}
	if now.Before(e.lockUntil) {
		return true
	}
	if !e.lockUntil.IsZero() {
		// 锁定过期，重置
		delete(l.entries, loginKey(username, ip))
	}
	return false
}

// recordFail 记录失败
func (l *loginFailCounter) recordFail(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	k := loginKey(username, ip)
	e, ok := l.entries[k]
	if !ok || now.Sub(e.lastFail) > failTTL {
		e = &loginFailEntry{}
		l.entries[k] = e
	}
	e.fails++
	e.lastFail = now
	if e.fails >= maxLoginFails {
		e.lockUntil = now.Add(lockDuration)
	}
}

// recordSuccess 登录成功清除
func (l *loginFailCounter) recordSuccess(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, loginKey(username, ip))
}

// Login 用户登录
func Login(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
			return
		}
		if req.Username == "" || req.Password == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "用户名和密码不能为空"})
			return
		}

		ip := clientIP(r)
		// 登录限流：锁定检查（按用户名+IP）
		if loginLimiter.checkLocked(req.Username, ip) {
			middleware.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "登录失败次数过多，账号已锁定，请10分钟后再试"})
			return
		}

		var user models.User
		var deptName, roleName, roleCode, phone sql.NullString
		var tokenVersion int
		err := database.DB.QueryRow(
			`SELECT u.id, u.username, u.password_hash, u.real_name, u.phone, u.department_id, d.name, u.role_id, r.name, r.code, u.status, u.token_version
			 FROM users u
			 LEFT JOIN departments d ON u.department_id = d.id
			 LEFT JOIN roles r ON u.role_id = r.id
			 WHERE u.username = ?`, req.Username).
			Scan(&user.ID, &user.Username, &user.PasswordHash, &user.RealName, &phone,
				&user.DepartmentID, &deptName, &user.RoleID, &roleName, &roleCode,
				&user.Status, &tokenVersion)
		if err == sql.ErrNoRows {
			// 用户不存在也执行一次 bcrypt，避免通过响应时间枚举用户名
			database.HashPassword(req.Password)
			loginLimiter.recordFail(req.Username, ip)
			logWithUser(0, req.Username, "登录", "登录", "登录失败：用户名或密码错误", clientIP(r))
			middleware.JSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
			return
		}
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统错误"})
			return
		}
		user.Phone = phone.String
		user.Department = deptName.String
		user.RoleName = roleName.String
		user.RoleCode = roleCode.String
		user.Permissions = middleware.RolePermissions(user.RoleCode)
		// 先校验密码再提示禁用：避免不知道密码的人借“账号已被禁用”提示探测账号是否存在
		if !database.CheckPassword(user.PasswordHash, req.Password) {
			loginLimiter.recordFail(req.Username, ip)
			logWithUser(user.ID, user.Username, "登录", "登录", "登录失败：用户名或密码错误", ip)
			middleware.JSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
			return
		}
		if user.Status != 1 {
			logWithUser(user.ID, user.Username, "登录", "登录", "登录失败：账号已被禁用", ip)
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "账号已被禁用"})
			return
		}

		// 登录成功，清除失败记录
		loginLimiter.recordSuccess(req.Username, ip)
		logWithUser(user.ID, user.Username, "登录", "登录", "登录成功", clientIP(r))

		token, err := auth.GenerateToken(user.ID, user.Username, user.RealName, user.RoleCode, tokenVersion)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "生成令牌失败"})
			return
		}

		resp := models.LoginResponse{
			Token: token,
			User:  user,
		}
		resp.User.PasswordHash = ""

		// 判断是否使用默认密码（需强制修改）：写入服务端标记，由认证中间件强制执行
		resp.MustChange = isDefaultPassword(req.Password)
		flag := 0
		if resp.MustChange {
			flag = 1
		}
		database.DB.Exec("UPDATE users SET must_change_password = ? WHERE id = ?", flag, user.ID)
		middleware.InvalidateTokenVersion(user.ID)

		middleware.JSON(w, http.StatusOK, resp)
	}
}

// Logout 退出登录：递增令牌版本，使该用户所有旧令牌立即失效
func Logout(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	if userID != 0 {
		database.DB.Exec("UPDATE users SET token_version = token_version + 1 WHERE id = ?", userID)
		middleware.InvalidateTokenVersion(userID)
		logWithUser(userID, "", "登录", "登录", "退出登录", clientIP(r))
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已退出登录"})
}

// GetProfile 获取当前用户信息
func GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var user models.User
	var phone, deptName, roleName, roleCode sql.NullString
	err := database.DB.QueryRow(
		`SELECT u.id, u.username, u.real_name, u.phone, u.department_id, d.name, u.role_id, r.name, r.code
		 FROM users u
		 LEFT JOIN departments d ON u.department_id = d.id
		 LEFT JOIN roles r ON u.role_id = r.id
		 WHERE u.id = ?`, userID).
		Scan(&user.ID, &user.Username, &user.RealName, &phone,
			&user.DepartmentID, &deptName, &user.RoleID, &roleName, &roleCode)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	user.Phone = phone.String
	user.Department = deptName.String
	user.RoleName = roleName.String
	user.RoleCode = roleCode.String
	user.Permissions = middleware.RolePermissions(user.RoleCode)
	middleware.JSON(w, http.StatusOK, user)
}

// ChangePassword 修改密码
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if len(req.NewPassword) < 8 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "新密码至少8位"})
		return
	}
	// bcrypt 只支持 72 字节以内，超长会加密失败；此前错误被忽略导致密码被清空、账号无法登录
	if len(req.NewPassword) > 72 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "新密码过长（不超过 72 个字符，汉字按 3 个计）"})
		return
	}
	if isDefaultPassword(req.NewPassword) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "新密码不能使用系统默认密码"})
		return
	}
	var hash string
	err := database.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	if !database.CheckPassword(hash, req.OldPassword) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "原密码错误"})
		return
	}
	newHash, err := database.HashPassword(req.NewPassword)
	if err != nil || newHash == "" {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "密码加密失败，请重试"})
		return
	}
	if _, err := database.DB.Exec("UPDATE users SET password_hash = ?, token_version = token_version + 1, must_change_password = 0, updated_at = ? WHERE id = ?", newHash, time.Now(), userID); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败，请重试"})
		return
	}
	middleware.InvalidateTokenVersion(userID)
	logOperation(r, "认证", "修改", "修改本人登录密码")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "密码修改成功"})
}

// ListUsers 用户列表（管理员）
func ListUsers(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	query := `SELECT u.id, u.username, u.real_name, u.phone, u.department_id, d.name, u.role_id, r.name, u.status, u.created_at
			  FROM users u
			  LEFT JOIN departments d ON u.department_id = d.id
			  LEFT JOIN roles r ON u.role_id = r.id`
	where := ` WHERE 1=1`
	args := []interface{}{}
	if keyword != "" {
		where += ` AND (u.real_name LIKE ? OR u.username LIKE ?)`
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	// 按科室筛选（左侧科室树）
	if deptID, err := strconv.ParseInt(r.URL.Query().Get("department_id"), 10, 64); err == nil && deptID > 0 {
		where += ` AND u.department_id = ?`
		args = append(args, deptID)
	}
	rows, err := database.DB.Query(query+where+` ORDER BY u.id DESC`, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		var phone, deptName, roleName sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.RealName, &phone, &u.DepartmentID, &deptName,
			&u.RoleID, &roleName, &u.Status, &u.CreatedAt); err != nil {
			continue
		}
		u.Phone = phone.String
		u.Department = deptName.String
		u.RoleName = roleName.String
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": users})
}

// CreateUser 创建用户
func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.User
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Username == "" || req.RealName == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "用户名和姓名必填"})
		return
	}
	var newRoleCode string
	if err := database.DB.QueryRow("SELECT code FROM roles WHERE id = ?", req.RoleID).Scan(&newRoleCode); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择有效角色"})
		return
	}
	if operatorRole, _ := r.Context().Value(middleware.ContextRoleCode).(string); operatorRole != "admin" && newRoleCode == "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅系统管理员可以创建管理员账号"})
		return
	}
	password := "123456" // 默认初始密码
	hash, err := database.HashPassword(password)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	// 用户 ID 不再复用（自增序列单调递增），且初始令牌版本取随机值：
	// 即使历史上曾复用过 ID，被删除用户残留的旧令牌也无法在新用户身上复活
	_, err = database.DB.Exec(
		`INSERT INTO users (username, password_hash, real_name, phone, department_id, role_id, status, token_version, must_change_password)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, 1)`,
		req.Username, hash, req.RealName, req.Phone, req.DepartmentID, req.RoleID, randomTokenVersion())
	if err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "创建失败，用户名可能已存在"})
		return
	}
	logOperation(r, "用户管理", "新增", "新增用户「"+req.RealName+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "创建成功，初始密码 123456"})
}

// UpdateUser 更新用户
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	operatorRole, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req struct {
		ID           int64  `json:"id"`
		RealName     string `json:"real_name"`
		Phone        string `json:"phone"`
		DepartmentID int64  `json:"department_id"`
		RoleID       int64  `json:"role_id"`
		Status       *int   `json:"status"` // 未传时保持原状态（此前缺省为 0 会把账号误禁用）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少用户ID"})
		return
	}
	// 读取原状态/角色，用于权限保护与判断是否需要使旧令牌失效
	var oldStatus int
	var oldRoleID int64
	var oldRoleCode sql.NullString
	if err := database.DB.QueryRow(
		"SELECT u.status, u.role_id, r.code FROM users u LEFT JOIN roles r ON u.role_id = r.id WHERE u.id = ?", req.ID).
		Scan(&oldStatus, &oldRoleID, &oldRoleCode); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	newStatus := oldStatus
	if req.Status != nil {
		newStatus = *req.Status
	}
	if newStatus != 0 && newStatus != 1 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "账号状态不合法"})
		return
	}
	if req.RoleID == 0 {
		req.RoleID = oldRoleID
	}
	var newRoleCode string
	if err := database.DB.QueryRow("SELECT code FROM roles WHERE id = ?", req.RoleID).Scan(&newRoleCode); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "角色不存在"})
		return
	}
	if msg := userChangeDenied(operatorID, operatorRole, req.ID, oldRoleCode.String, newRoleCode, oldStatus, newStatus); msg != "" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg})
		return
	}
	_, err := database.DB.Exec(
		"UPDATE users SET real_name = ?, phone = ?, department_id = ?, role_id = ?, status = ?, updated_at = ? WHERE id = ?",
		req.RealName, req.Phone, req.DepartmentID, req.RoleID, newStatus, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	// 禁用或角色变更时，使该用户旧令牌立即失效；其余变更（姓名等）也刷新认证缓存
	if newStatus != oldStatus || req.RoleID != oldRoleID {
		database.DB.Exec("UPDATE users SET token_version = token_version + 1 WHERE id = ?", req.ID)
	}
	middleware.InvalidateTokenVersion(req.ID)
	logOperation(r, "用户管理", "修改", "修改用户「"+req.RealName+"」的信息")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// userChangeDenied 用户变更保护规则，返回拒绝原因（放行为空）：
//   - 不能禁用自己、不能修改自己的角色；
//   - 非管理员（仅持有 user.manage）不得授予管理员角色，也不得修改管理员账号；
//   - 必须至少保留一个启用状态的管理员。
func userChangeDenied(operatorID int64, operatorRole string, targetID int64, oldRole, newRole string, oldStatus, newStatus int) string {
	if targetID == operatorID {
		if newStatus != 1 {
			return "不能禁用当前登录账号"
		}
		if newRole != oldRole {
			return "不能修改当前登录账号的角色"
		}
	}
	if operatorRole != "admin" && (oldRole == "admin" || newRole == "admin") {
		return "仅系统管理员可以授予管理员角色或修改管理员账号"
	}
	if oldRole == "admin" && oldStatus == 1 && (newRole != "admin" || newStatus != 1) {
		var others int
		database.DB.QueryRow(`SELECT COUNT(*) FROM users u JOIN roles r ON u.role_id = r.id
			WHERE r.code = 'admin' AND u.status = 1 AND u.id != ?`, targetID).Scan(&others)
		if others == 0 {
			return "必须至少保留一个启用状态的管理员"
		}
	}
	return ""
}

// ResetPassword 重置密码
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少用户ID"})
		return
	}
	var targetName string
	var targetRole sql.NullString
	database.DB.QueryRow("SELECT u.real_name, r.code FROM users u LEFT JOIN roles r ON u.role_id = r.id WHERE u.id = ?", req.ID).Scan(&targetName, &targetRole)
	if operatorRole, _ := r.Context().Value(middleware.ContextRoleCode).(string); operatorRole != "admin" && targetRole.String == "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅系统管理员可以重置管理员密码"})
		return
	}
	hash, err := database.HashPassword("123456")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "重置失败"})
		return
	}
	res, err := database.DB.Exec("UPDATE users SET password_hash = ?, token_version = token_version + 1, must_change_password = 1, updated_at = ? WHERE id = ?", hash, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "重置失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	middleware.InvalidateTokenVersion(req.ID)
	logOperation(r, "用户管理", "修改", "重置用户「"+targetName+"」的登录密码")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "密码已重置为 123456"})
}

// ListRoles 角色列表
func ListRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, code, description FROM roles ORDER BY id")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	roles := []models.Role{}
	for rows.Next() {
		var role models.Role
		var description sql.NullString
		if err := rows.Scan(&role.ID, &role.Name, &role.Code, &description); err != nil {
			continue
		}
		role.Description = description.String
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": roles})
}

// ListDepartments 部门列表
func ListDepartments(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort FROM departments ORDER BY sort")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	depts := []models.Department{}
	for rows.Next() {
		var d models.Department
		if err := rows.Scan(&d.ID, &d.Name, &d.ParentID, &d.Sort); err != nil {
			continue
		}
		depts = append(depts, d)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": depts})
}

// CreateDepartment 添加部门（管理员）
func CreateDepartment(w http.ResponseWriter, r *http.Request) {
	var req models.Department
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Name == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "部门名称不能为空"})
		return
	}
	var cnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM departments WHERE name = ?", req.Name).Scan(&cnt)
	if cnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "部门名称已存在"})
		return
	}
	var maxSort int
	database.DB.QueryRow("SELECT COALESCE(MAX(sort), 0) FROM departments").Scan(&maxSort)
	res, err := database.DB.Exec("INSERT INTO departments (name, parent_id, sort) VALUES (?, ?, ?)",
		req.Name, req.ParentID, maxSort+1)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "添加失败"})
		return
	}
	id, _ := res.LastInsertId()
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "message": "添加成功"})
}

// UpdateDepartment 修改部门（管理员）
func UpdateDepartment(w http.ResponseWriter, r *http.Request) {
	var req models.Department
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少部门ID"})
		return
	}
	if req.Name == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "部门名称不能为空"})
		return
	}
	var cnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM departments WHERE name = ? AND id != ?", req.Name, req.ID).Scan(&cnt)
	if cnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "部门名称已存在"})
		return
	}
	_, err := database.DB.Exec("UPDATE departments SET name = ? WHERE id = ?", req.Name, req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
}

// DeleteDepartment 删除部门（管理员）
// 若该部门下仍有用户或通讯录引用，则禁止删除，避免产生孤立数据
func DeleteDepartment(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少部门ID"})
		return
	}
	var userCnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE department_id = ?", id).Scan(&userCnt)
	if userCnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该部门下仍有用户，请先调整用户部门后再删除"})
		return
	}
	var contactCnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM contacts WHERE department_id = ?", id).Scan(&contactCnt)
	if contactCnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该部门下仍有通讯录联系人，请先调整后再删除"})
		return
	}
	_, err := database.DB.Exec("DELETE FROM departments WHERE id = ?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	// 重置自增序列，使被删除的最大 ID 可被复用
	resetAutoIncrement()
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// DeleteUser 真删除用户（管理员）
// 从数据库物理删除，释放 ID，同时清理该用户在其他表中的关联数据
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)

	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少用户ID"})
		return
	}
	// 不能删除自己
	if id == operatorID {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不能删除当前登录账号"})
		return
	}
	// 检查用户是否存在，并取姓名用于日志
	var exists int
	var targetName string
	err := database.DB.QueryRow("SELECT COUNT(*), COALESCE(MAX(real_name),'') FROM users WHERE id=?", id).Scan(&exists, &targetName)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	if exists == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	// 防止删除最后一个管理员
	var roleCode string
	var adminCount int
	database.DB.QueryRow("SELECT code FROM roles WHERE id=(SELECT role_id FROM users WHERE id=?)", id).Scan(&roleCode)
	if operatorRole, _ := r.Context().Value(middleware.ContextRoleCode).(string); operatorRole != "admin" && roleCode == "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅系统管理员可以删除管理员账号"})
		return
	}
	if roleCode == "admin" {
		database.DB.QueryRow("SELECT COUNT(*) FROM users u JOIN roles r ON u.role_id=r.id WHERE r.code='admin' AND u.status=1 AND u.id != ?", id).Scan(&adminCount)
		if adminCount == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不能删除最后一个管理员"})
			return
		}
	}

	// 清理该用户的关联数据并物理删除用户，整体事务保证一致性
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	defer tx.Rollback()
	// 个人维度的私有记录随用户物理删除
	personalTables := []string{
		"attendances", "leave_records", "circulation_records",
		"duty_schedules", "overtime_records", "annual_leave_configs",
	}
	for _, t := range personalTables {
		if _, err := tx.Exec("DELETE FROM "+t+" WHERE user_id=?", id); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
	}
	// 单位公共档案与历史业务资产（公文、常委大事记、大事记、周总结、公共资料、日程备忘、用车台账、会务、征求意见、材料下发）
	// 严禁级联物理删除，统一将其录入人转移给当前操作管理员
	archiveTransfers := []struct {
		table string
		col   string
	}{
		{"incoming_docs", "registrar_id"},
		{"standing_committee_events", "created_by"},
		{"major_events", "created_by"},
		{"weekly_summaries", "created_by"},
		{"study_materials", "publisher_id"},
		{"calendar_tasks", "created_by"},
		{"vehicle_applies", "reporter_id"},
		{"meetings", "created_by"},
		{"solicits", "created_by"},
		{"dispatches", "created_by"},
	}
	for _, at := range archiveTransfers {
		if _, err := tx.Exec("UPDATE "+at.table+" SET "+at.col+"=? WHERE "+at.col+"=?", operatorID, id); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id=?", id); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}

	// 立即注销该用户的令牌版本缓存，确保历史 JWT 令牌立即失效
	middleware.InvalidateTokenVersion(id)

	// 重置业务表自增序列，保证业务编号不跳号（users / attachments 除外，见 resetAutoIncrement）
	resetAutoIncrement()

	logOperation(r, "用户管理", "删除", "删除用户「"+targetName+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "用户已删除"})
}

// resetAutoIncrement 将业务表自增序列重置为当前最大 ID，实现删除后编号不跳号。
// users 与 attachments 严禁复用 ID：前者会让被删用户的旧令牌复活，后者会让旧的 /api/uploads/{id} 引用指向新文件。
func resetAutoIncrement() {
	tables := []string{"departments", "attendances", "leave_records", "vehicles", "vehicle_applies",
		"contacts", "duty_schedules", "incoming_docs", "circulation_records",
		"study_materials", "calendar_tasks", "standing_committee_events",
		"major_events", "weekly_summaries", "overtime_records", "annual_leave_configs",
		"meetings", "meeting_registrations", "solicits", "solicit_feedbacks", "dispatches", "dispatch_receipts"}
	tx, err := database.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	for _, t := range tables {
		// 将序列设为当前最大 ID（若表为空则为 0）
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			tx.Rollback()
			return
		}
		if _, err := tx.Exec("INSERT OR REPLACE INTO sqlite_sequence (name, seq) VALUES (?, (SELECT COALESCE(MAX(id),0) FROM "+t+"))", t); err != nil {
			tx.Rollback()
			return
		}
	}
	tx.Commit()
}

// 分页参数
type pageParam struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// parsePage 解析分页参数，默认第1页，每页20条
func parsePage(r *http.Request) pageParam {
	page := 1
	pageSize := 20
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}
	return pageParam{Page: page, PageSize: pageSize}
}

// paginateResult 构造分页返回结构
func paginateResult(list interface{}, total, page, pageSize int) map[string]interface{} {
	return map[string]interface{}{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}
}

// randomTokenVersion 新建账号的初始令牌版本（随机正整数）
func randomTokenVersion() int {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return int(time.Now().UnixNano() & 0x3fffffff)
	}
	return int(binary.BigEndian.Uint32(b)&0x3fffffff) + 1
}

// isDefaultPassword 是否为系统默认密码（初始/重置密码）
func isDefaultPassword(p string) bool {
	return p == "admin123" || p == "123456"
}
