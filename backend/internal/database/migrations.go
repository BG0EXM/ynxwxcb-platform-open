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

// migrateV21 版本21：新增库房物资模块（物资台账、领用单、库存流水、门口扫码设置）
func migrateV21() error {
	stmts := []string{
		// 物资台账：kind=consumable 耗材（领用不归还）/ returnable 可归还物品（借用后归还）
		`CREATE TABLE IF NOT EXISTS warehouse_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			category TEXT DEFAULT '',
			spec TEXT DEFAULT '',
			unit TEXT DEFAULT '个',
			kind TEXT DEFAULT 'consumable',
			stock INTEGER DEFAULT 0,
			warn_qty INTEGER DEFAULT 0,
			location TEXT DEFAULT '',
			status INTEGER DEFAULT 1,
			sort INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name, spec)
		)`,
		// 领用单：一次扫码提交一张；user_id=0 为名单外“其他人员”
		`CREATE TABLE IF NOT EXISTS warehouse_slips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slip_no TEXT NOT NULL UNIQUE,
			user_id INTEGER DEFAULT 0,
			person_name TEXT NOT NULL,
			department TEXT DEFAULT '',
			phone TEXT DEFAULT '',
			is_external INTEGER DEFAULT 0,
			purpose TEXT DEFAULT '',
			source TEXT DEFAULT 'public',
			operator_id INTEGER DEFAULT 0,
			ip TEXT DEFAULT '',
			submit_id TEXT DEFAULT '',
			voided INTEGER DEFAULT 0,
			void_reason TEXT DEFAULT '',
			voided_by TEXT DEFAULT '',
			voided_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 库存流水：qty 为库存变动量（入库/归还/冲回为正，领用/借用为负，盘点可正可负）
		`CREATE TABLE IF NOT EXISTS warehouse_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL,
			slip_id INTEGER DEFAULT 0,
			kind TEXT NOT NULL,
			qty INTEGER NOT NULL,
			stock_after INTEGER DEFAULT 0,
			borrow_id INTEGER DEFAULT 0,
			returned_qty INTEGER DEFAULT 0,
			returned_at DATETIME,
			user_id INTEGER DEFAULT 0,
			person_name TEXT DEFAULT '',
			department TEXT DEFAULT '',
			operator TEXT DEFAULT '',
			remark TEXT DEFAULT '',
			voided INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 门口二维码设置（单行）
		`CREATE TABLE IF NOT EXISTS warehouse_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			public_token TEXT NOT NULL,
			paused INTEGER DEFAULT 0,
			notice TEXT DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_wh_slips_submit ON warehouse_slips(submit_id) WHERE submit_id <> ''",
		"CREATE INDEX IF NOT EXISTS idx_wh_slips_user ON warehouse_slips(user_id, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_wh_slips_created ON warehouse_slips(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_wh_records_item ON warehouse_records(item_id, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_wh_records_slip ON warehouse_records(slip_id)",
		"CREATE INDEX IF NOT EXISTS idx_wh_records_created ON warehouse_records(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_wh_records_borrow ON warehouse_records(kind, voided, user_id)",
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return err
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	if _, err := DB.Exec("INSERT OR IGNORE INTO warehouse_settings (id, public_token) VALUES (1, ?)", hex.EncodeToString(b)); err != nil {
		return err
	}

	// 新权限点：默认不授予任何角色（管理员旁路可用），由管理员在权限矩阵中授权办公室经办人
	if _, err := DB.Exec(`INSERT INTO permissions (code, name, module, sort) VALUES ('warehouse.manage', '库房物资管理（物资台账/领用记录/二维码/导出）', '库房物资', 107)
		ON CONFLICT(code) DO UPDATE SET name=excluded.name, module=excluded.module, sort=excluded.sort`); err != nil {
		return err
	}
	return nil
}
