package database

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
