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

// migrateV22 版本22：新增 AI 写作助手模块（模型设置、用户余额、加款流水、用量明细、提示词库）
func migrateV22() error {
	stmts := []string{
		// 模型设置（单行）：provider=local 本地模型 / remote 网络接口；
		// 单价单位为“元/百万 token”，offpeak_ratio 为空闲时段相对高峰价的折扣系数（1=不打折，DeepSeek 为 0.5）；
		// 余额与金额一律以“微元”（1 元 = 1,000,000）整数存储，避免浮点累计误差
		`CREATE TABLE IF NOT EXISTS ai_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			enabled INTEGER DEFAULT 0,
			provider TEXT DEFAULT 'remote',
			base_url TEXT DEFAULT '',
			api_key_enc TEXT DEFAULT '',
			model TEXT DEFAULT '',
			price_in_hit REAL DEFAULT 0,
			price_in_miss REAL DEFAULT 0,
			price_out REAL DEFAULT 0,
			offpeak_ratio REAL DEFAULT 1,
			max_output_tokens INTEGER DEFAULT 2048,
			max_input_chars INTEGER DEFAULT 12000,
			warn_balance_micro INTEGER DEFAULT 2000000,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 用户余额账户
		`CREATE TABLE IF NOT EXISTS ai_accounts (
			user_id INTEGER PRIMARY KEY,
			balance_micro INTEGER DEFAULT 0,
			total_topup_micro INTEGER DEFAULT 0,
			total_spent_micro INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 加款/调整流水：kind=topup 加款 / adjust 扣减更正；amount_micro 带正负号
		`CREATE TABLE IF NOT EXISTS ai_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			amount_micro INTEGER NOT NULL,
			balance_after_micro INTEGER DEFAULT 0,
			operator TEXT DEFAULT '',
			remark TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 每次调用的用量与扣费明细（不保存对话正文）
		`CREATE TABLE IF NOT EXISTS ai_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			scene TEXT DEFAULT '',
			provider TEXT DEFAULT '',
			model TEXT DEFAULT '',
			prompt_tokens INTEGER DEFAULT 0,
			hit_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			cost_micro INTEGER DEFAULT 0,
			status TEXT DEFAULT 'ok',
			estimated INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 提示词库（场景）：scene_key 用于业务页嵌入按钮定位，builtin=1 为系统预置（可改不可删）
		`CREATE TABLE IF NOT EXISTS ai_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scene_key TEXT DEFAULT '',
			name TEXT NOT NULL,
			category TEXT DEFAULT '',
			system_prompt TEXT NOT NULL,
			hint TEXT DEFAULT '',
			placeholder TEXT DEFAULT '',
			sort INTEGER DEFAULT 0,
			status INTEGER DEFAULT 1,
			builtin INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		"CREATE INDEX IF NOT EXISTS idx_ai_usage_user ON ai_usage(user_id, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_ai_usage_created ON ai_usage(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_ai_ledger_user ON ai_ledger(user_id, created_at)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_prompts_scene ON ai_prompts(scene_key) WHERE scene_key <> ''",
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return err
		}
	}
	if _, err := DB.Exec("INSERT OR IGNORE INTO ai_settings (id) VALUES (1)"); err != nil {
		return err
	}

	// 预置提示词（INSERT OR IGNORE：scene_key 唯一，重复执行不覆盖管理员改过的内容）
	const rule = "要求：语言庄重规范、条理清晰，符合党政机关公文习惯；不得虚构文号、数据、人名、日期和事实，缺少的信息用【】标出请使用者补充；只输出正文，不加解释。"
	prompts := []struct{ key, name, cat, sys, hint, ph string }{
		{"chat_notice", "拟写通知", "公文写作", "你是县委宣传部的资深文秘，请根据用户提供的事由、时间、地点、对象和要求，拟写一份规范的通知。" + rule, "写清事由、时间、地点、参加对象和要求", "例如：10月28日上午9点在县委三楼会议室召开全县意识形态工作推进会，各乡镇分管领导参加……"},
		{"chat_summary", "工作总结", "公文写作", "你是县委宣传部的资深文秘，请把用户提供的工作要点整理成一份结构清晰的工作总结，分“主要做法”“存在问题”“下一步打算”等层次。" + rule, "粘贴本阶段的工作要点即可", "例如：本周完成了……开展了……"},
		{"chat_news", "新闻信息稿", "公文写作", "你是县委宣传部的资深文秘，请根据用户提供的活动素材撰写一篇简明的新闻信息稿，包含标题、导语和正文。" + rule, "提供活动时间、地点、内容和成效", "例如：近日，伊宁县组织开展……"},
		{"chat_speech", "讲话提纲", "公文写作", "你是县委宣传部的资深文秘，请根据用户给出的会议主题和要点，拟写一份讲话提纲，分三到五个层次，每层给出小标题和要点。" + rule, "写明会议主题、讲话人身份和要强调的重点", "例如：在全县宣传思想文化工作会议上的讲话……"},
		{"chat_polish", "公文润色", "公文写作", "你是县委宣传部的资深文秘，请对用户给出的文字进行润色，使其用词准确、语气得体、逻辑通顺，保持原意不变，不增删事实。" + rule, "粘贴需要润色的文字", "粘贴需要润色的文字……"},
		{"chat_free", "自由提问", "通用", "你是县委宣传部的办公助手，请用通俗准确的中文回答用户的问题。涉及事实、数据、政策时如无把握，请明确说明需要核实，不得编造。", "任何办公写作问题都可以问", "请输入你的问题……"},
		{"meeting_notice", "会议通知润色", "业务嵌入", "你是县委宣传部的资深文秘，请对用户给出的会议通知文案进行润色，使其格式规范、用语得体，保留时间、地点、参会范围等全部原始信息，不得改动或虚构。" + rule, "对一键生成的通知文案做润色", ""},
		{"solicit_text", "征求意见说明文案", "业务嵌入", "你是县委宣传部的资深文秘，请根据用户给出的公文草案名称和要求，拟写一段发给各单位的征求意见说明，写明征求事项、反馈方式和截止时间。" + rule, "根据标题和要求生成说明文案", ""},
		{"solicit_digest", "意见汇编归并", "业务嵌入", "你是县委宣传部的资深文秘，用户会给出各单位对同一份文稿的修改意见。请按条款或主题归并同类意见，去除重复，每条注明提出单位，并最后给出不超过 100 字的总体概述。不得遗漏有实质内容的意见，不得改变意见原意。", "把各单位意见汇编成一份归并后的清单", ""},
		{"dispatch_text", "材料下发说明文案", "业务嵌入", "你是县委宣传部的资深文秘，请根据用户给出的材料名称和要求，拟写一段发给各单位的材料下发说明，写明材料用途、查收要求和注意事项。" + rule, "根据标题和要求生成说明文案", ""},
		{"weekly_polish", "每周总结润色", "业务嵌入", "你是县委宣传部的资深文秘，请对用户给出的本周工作总结进行润色，使表述精炼、条理清晰、用语规范，保持原有事实不变。" + rule, "润色本周工作总结", ""},
		{"event_polish", "大事记润色", "业务嵌入", "你是县委宣传部的资深文秘，请把用户给出的大事记条目改写为简明规范的大事记表述：一句话写清时间、事项、主要成效，语言精炼客观。" + rule, "把条目改写成规范的大事记表述", ""},
		{"doc_summary", "公文摘要", "业务嵌入", "你是县委宣传部的资深文秘，请对用户给出的公文内容写一段不超过 150 字的摘要，说明发文单位、主要事项和需要办理的要求。不得虚构内容。", "提取公文要点，便于拟办批示", ""},
	}
	for i, p := range prompts {
		if _, err := DB.Exec(`INSERT OR IGNORE INTO ai_prompts (scene_key, name, category, system_prompt, hint, placeholder, sort, builtin) VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
			p.key, p.name, p.cat, p.sys, p.hint, p.ph, (i+1)*10); err != nil {
			return err
		}
	}

	// 新权限点：默认不授予任何角色（管理员旁路可用），由管理员在权限矩阵中勾选
	if _, err := DB.Exec(`INSERT INTO permissions (code, name, module, sort) VALUES ('ai.use', 'AI 写作助手（对话与业务页辅助写作，按用量扣费）', 'AI助手', 108)
		ON CONFLICT(code) DO UPDATE SET name=excluded.name, module=excluded.module, sort=excluded.sort`); err != nil {
		return err
	}
	return nil
}
