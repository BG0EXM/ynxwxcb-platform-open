package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
)

// migrateV19 版本19：追加缺失的高频索引迁移
func migrateV19() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_users_dept ON users(department_id)",
		"CREATE INDEX IF NOT EXISTS idx_users_role ON users(role_id)",
		"CREATE INDEX IF NOT EXISTS idx_circ_user ON circulation_records(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_study_cat ON study_materials(category)",
		"CREATE INDEX IF NOT EXISTS idx_incoming_dept ON incoming_docs(assigned_department)",
		"CREATE INDEX IF NOT EXISTS idx_att_composite ON attendances(user_id, attend_date)",
	}
	for _, sql := range indexes {
		if _, err := DB.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}

// migrateV20 版本20：公开链接随机令牌、收回乡镇通讯员征求意见/材料下发管理权限、存量文件引用绑定归属
func migrateV20() error {
	// 1. 会务/征求意见/材料下发增加 128 位随机公开令牌，旧数字 ID 链接作废
	for _, table := range []string{"meetings", "solicits", "dispatches"} {
		if !hasColumn(table, "public_token") {
			if _, err := DB.Exec("ALTER TABLE " + table + " ADD COLUMN public_token TEXT"); err != nil {
				return err
			}
		}
		rows, err := DB.Query("SELECT id FROM " + table + " WHERE public_token IS NULL OR public_token = ''")
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				return err
			}
			if _, err := DB.Exec("UPDATE "+table+" SET public_token=? WHERE id=?", hex.EncodeToString(b), id); err != nil {
				return err
			}
		}
		if _, err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_" + table + "_public_token ON " + table + "(public_token)"); err != nil {
			return err
		}
	}

	// 2. 默认密码强制修改标记（由服务端强制执行，不再仅靠前端判断）
	if !hasColumn("users", "must_change_password") {
		if _, err := DB.Exec("ALTER TABLE users ADD COLUMN must_change_password INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}

	// 2.1 会务公开报名单位编辑凭证（首次报名签发，仅保存哈希；修改/移除/取消须持凭证）
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS meeting_unit_tokens (
		meeting_id INTEGER NOT NULL,
		unit TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (meeting_id, unit)
	)`); err != nil {
		return err
	}

	// 3. 乡镇通讯员收回征求意见/材料下发管理权限（科室用户与分管领导保留）
	if _, err := DB.Exec(`DELETE FROM role_permissions
		WHERE permission_code IN ('solicit.manage', 'dispatch.manage')
		AND role_id IN (SELECT id FROM roles WHERE code = 'reporter')`); err != nil {
		return err
	}

	// 4. 存量征求意见/材料下发引用的附件绑定到对应记录，防止被其他记录重复引用
	binds := []string{
		"SELECT id, pdf_path FROM solicits", "SELECT id, word_path FROM solicits",
		"SELECT id, pdf_path FROM dispatches", "SELECT id, attachment_path FROM dispatches",
	}
	for _, q := range binds {
		rows, err := DB.Query(q)
		if err != nil {
			return err
		}
		type ref struct {
			owner int64
			att   int64
		}
		var refs []ref
		for rows.Next() {
			var owner int64
			var p sql.NullString
			if err := rows.Scan(&owner, &p); err != nil {
				rows.Close()
				return err
			}
			if strings.HasPrefix(p.String, "/api/uploads/") {
				if att, err := strconv.ParseInt(strings.TrimPrefix(p.String, "/api/uploads/"), 10, 64); err == nil && att > 0 {
					refs = append(refs, ref{owner, att})
				}
			}
		}
		rows.Close()
		for _, r := range refs {
			if _, err := DB.Exec("UPDATE attachments SET owner_id=? WHERE id=? AND owner_id=0", r.owner, r.att); err != nil {
				return err
			}
		}
	}
	return nil
}
