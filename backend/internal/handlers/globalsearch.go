package handlers

import (
	"net/http"
	"strings"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// GlobalSearchResultItem 全局搜索单项结果
type GlobalSearchResultItem struct {
	ID       int64  `json:"id,omitempty"`
	Type     string `json:"type"` // "contact", "incoming", "study", "meeting", "nav"
	Title    string `json:"title"`
	SubTitle string `json:"sub_title,omitempty"`

	// 通讯录字段
	Name       string `json:"name,omitempty"`
	Position   string `json:"position,omitempty"`
	Department string `json:"department,omitempty"`
	Phone      string `json:"phone,omitempty"`

	// 收文公文字段
	DocNo      string `json:"doc_no,omitempty"`
	SourceUnit string `json:"source_unit,omitempty"`

	// 资料字段
	Category string `json:"category,omitempty"`

	// 会议字段
	Time        string `json:"time,omitempty"`
	MeetingTime string `json:"meeting_time,omitempty"`
	Location    string `json:"location,omitempty"`

	// 页面导航指令
	Path string `json:"path,omitempty"`
	Icon string `json:"icon,omitempty"`
}

// 预定义系统导航菜单
type navMenuItem struct {
	Title    string
	Path     string
	Icon     string
	Category string
	Keywords []string
}

var systemNavMenus = []navMenuItem{
	{
		Title:    "工作台",
		Path:     "/dashboard",
		Icon:     "Odometer",
		Category: "基础功能",
		Keywords: []string{"工作台", "首页", "概览", "看板", "dashboard", "shouye", "gongzuotai"},
	},
	{
		Title:    "收文管理",
		Path:     "/incoming",
		Icon:     "FolderOpened",
		Category: "业务办理",
		Keywords: []string{"收文", "公文", "来文", "呈批", "批转", "传阅", "收发文", "incoming", "shouwen"},
	},
	{
		Title:    "公车管理",
		Path:     "/vehicles",
		Icon:     "Van",
		Category: "业务办理",
		Keywords: []string{"公车", "车辆", "用车", "派车", "驾驶员", "车辆台账", "vehicles", "gongche"},
	},
	{
		Title:    "值守排班",
		Path:     "/duty",
		Icon:     "AlarmClock",
		Category: "业务办理",
		Keywords: []string{"排班", "值班", "值守", "值班表", "大网格", "duty", "paiban", "zhiban"},
	},
	{
		Title:    "工作日历",
		Path:     "/calendar",
		Icon:     "Calendar",
		Category: "业务办理",
		Keywords: []string{"日历", "日程", "工作日历", "行事历", "安排", "calendar", "rili", "richeng"},
	},
	{
		Title:    "会务管理",
		Path:     "/meetings",
		Icon:     "OfficeBuilding",
		Category: "业务办理",
		Keywords: []string{"会议", "会务", "报名", "参会", "会议管理", "meetings", "huiyi", "huiwu"},
	},
	{
		Title:    "通讯录",
		Path:     "/contacts",
		Icon:     "Phone",
		Category: "业务办理",
		Keywords: []string{"通讯录", "电话", "干部", "联系方式", "联系人", "科室", "contacts", "tongxunlu", "dianhua"},
	},
	{
		Title:    "考勤点到",
		Path:     "/attendance",
		Icon:     "CircleCheck",
		Category: "考勤休假",
		Keywords: []string{"考勤", "点到", "打卡", "出勤", "签到", "迟到", "attendance", "kaoqin"},
	},
	{
		Title:    "请假管理",
		Path:     "/leave",
		Icon:     "Memo",
		Category: "考勤休假",
		Keywords: []string{"请假", "假条", "休假", "出差", "事假", "病假", "leave", "qingjia"},
	},
	{
		Title:    "加班管理",
		Path:     "/overtime",
		Icon:     "Clock",
		Category: "考勤休假",
		Keywords: []string{"加班", "调休", "补休", "加班工时", "overtime", "jiaban", "buxiu"},
	},
	{
		Title:    "年休假管理",
		Path:     "/annualleave",
		Icon:     "Sunny",
		Category: "考勤休假",
		Keywords: []string{"年假", "年休假", "休假", "带薪休假", "annualleave", "nianjia"},
	},
	{
		Title:    "大事记",
		Path:     "/reports",
		Icon:     "Collection",
		Category: "材料报送",
		Keywords: []string{"大事记", "纪事", "宣传要闻", "历程", "reports", "dashiji"},
	},
	{
		Title:    "每周工作总结",
		Path:     "/weekly",
		Icon:     "Document",
		Category: "材料报送",
		Keywords: []string{"每周工作总结", "周总结", "工作总结", "周报", "每周要事", "weekly", "zhoubao", "zongjie"},
	},
	{
		Title:    "公共资料",
		Path:     "/study",
		Icon:     "Reading",
		Category: "材料报送",
		Keywords: []string{"公共资料", "学习资料", "文件", "学习", "政策", "制度", "study", "ziliao", "xuexi"},
	},
	{
		Title:    "常委管理",
		Path:     "/standing",
		Icon:     "UserFilled",
		Category: "系统管理",
		Keywords: []string{"常委", "常委管理", "常委会", "常委工作", "standing", "changwei"},
	},
	{
		Title:    "用户管理",
		Path:     "/users",
		Icon:     "User",
		Category: "系统管理",
		Keywords: []string{"用户", "用户管理", "账号", "人员管理", "干部账号", "users", "yonghu"},
	},
	{
		Title:    "权限管理",
		Path:     "/permissions",
		Icon:     "Lock",
		Category: "系统管理",
		Keywords: []string{"权限", "权限管理", "角色", "权限分配", "permissions", "quanxian"},
	},
	{
		Title:    "操作日志",
		Path:     "/operation-logs",
		Icon:     "Files",
		Category: "系统管理",
		Keywords: []string{"操作日志", "日志", "审计", "记录", "logs", "rizhi"},
	},
	{
		Title:    "数据备份",
		Path:     "/backups",
		Icon:     "Coin",
		Category: "系统管理",
		Keywords: []string{"备份", "数据备份", "数据库备份", "还原", "backups", "beifen"},
	},
	{
		Title:    "个人中心",
		Path:     "/profile",
		Icon:     "Setting",
		Category: "个人中心",
		Keywords: []string{"个人中心", "密码", "修改密码", "个人设置", "profile", "mima"},
	},
}

// GlobalSearch 全局速查与指令接口 GET /api/global-search?q={keyword}
func GlobalSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	format := r.URL.Query().Get("format")

	// 参数校验：长度至少 1 个字符，如果为空返回空数组
	if len(q) < 1 {
		if format == "grouped" {
			middleware.JSON(w, http.StatusOK, map[string]interface{}{
				"contacts":   []GlobalSearchResultItem{},
				"incoming":   []GlobalSearchResultItem{},
				"study":      []GlobalSearchResultItem{},
				"meetings":   []GlobalSearchResultItem{},
				"navigation": []GlobalSearchResultItem{},
				"list":       []GlobalSearchResultItem{},
			})
			return
		}
		middleware.JSON(w, http.StatusOK, []GlobalSearchResultItem{})
		return
	}

	kw := "%" + q + "%"
	const limitPerCategory = 8

	contacts := []GlobalSearchResultItem{}
	incomingList := []GlobalSearchResultItem{}
	studyList := []GlobalSearchResultItem{}
	meetingList := []GlobalSearchResultItem{}
	navList := []GlobalSearchResultItem{}

	// 1. 页面导航快捷指令检索
	qLower := strings.ToLower(q)
	for _, menu := range systemNavMenus {
		if len(navList) >= limitPerCategory {
			break
		}
		matched := false
		if strings.Contains(strings.ToLower(menu.Title), qLower) || strings.Contains(strings.ToLower(menu.Path), qLower) {
			matched = true
		} else {
			for _, k := range menu.Keywords {
				if strings.Contains(strings.ToLower(k), qLower) || strings.Contains(qLower, strings.ToLower(k)) {
					matched = true
					break
				}
			}
		}
		if matched {
			navList = append(navList, GlobalSearchResultItem{
				Type:     "nav",
				Title:    menu.Title,
				Path:     menu.Path,
				Icon:     menu.Icon,
				SubTitle: "快速跳转 · " + menu.Category,
			})
		}
	}

	// 2. 通讯录/干部联系方式（contacts 表）：匹配 name, position, phone
	contactQuery := `SELECT c.id, c.name, COALESCE(c.position, ''), COALESCE(d.name, ''), COALESCE(c.phone, '')
		FROM contacts c
		LEFT JOIN departments d ON c.department_id = d.id
		WHERE c.name LIKE ? OR c.position LIKE ? OR c.phone LIKE ?
		ORDER BY c.sort ASC, c.id ASC
		LIMIT ?`
	if rows, err := database.DB.Query(contactQuery, kw, kw, kw, limitPerCategory); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var name, position, dept, phone string
			if err := rows.Scan(&id, &name, &position, &dept, &phone); err == nil {
				sub := dept
				if position != "" {
					if sub != "" {
						sub += " · " + position
					} else {
						sub = position
					}
				}
				if phone != "" {
					if sub != "" {
						sub += " (" + phone + ")"
					} else {
						sub = phone
					}
				}
				contacts = append(contacts, GlobalSearchResultItem{
					ID:         id,
					Type:       "contact",
					Title:      name,
					Name:       name,
					Position:   position,
					Department: dept,
					Phone:      phone,
					SubTitle:   sub,
				})
			}
		}
	}

	// 3. 收文公文（incoming_docs 表）：匹配 doc_no, title, source_unit (from_unit)
	incomingQuery := `SELECT id, COALESCE(NULLIF(doc_no, ''), NULLIF(from_doc_no, ''), receive_no, ''), title, COALESCE(from_unit, '')
		FROM incoming_docs
		WHERE title LIKE ? OR from_unit LIKE ? OR doc_no LIKE ? OR from_doc_no LIKE ? OR receive_no LIKE ?
		ORDER BY id DESC
		LIMIT ?`
	if rows, err := database.DB.Query(incomingQuery, kw, kw, kw, kw, kw, limitPerCategory); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var docNo, title, fromUnit string
			if err := rows.Scan(&id, &docNo, &title, &fromUnit); err == nil {
				sub := fromUnit
				if docNo != "" {
					if sub != "" {
						sub += " | 字号: " + docNo
					} else {
						sub = "字号: " + docNo
					}
				}
				incomingList = append(incomingList, GlobalSearchResultItem{
					ID:         id,
					Type:       "incoming",
					Title:      title,
					DocNo:      docNo,
					SourceUnit: fromUnit,
					SubTitle:   sub,
				})
			}
		}
	}

	// 4. 公共资料（study_materials 表）：匹配 title, category
	studyQuery := `SELECT id, title, COALESCE(category, '')
		FROM study_materials
		WHERE title LIKE ? OR category LIKE ?
		ORDER BY id DESC
		LIMIT ?`
	if rows, err := database.DB.Query(studyQuery, kw, kw, limitPerCategory); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var title, category string
			if err := rows.Scan(&id, &title, &category); err == nil {
				sub := "公共学习资料"
				if category != "" {
					sub = "资料分类: " + category
				}
				studyList = append(studyList, GlobalSearchResultItem{
					ID:       id,
					Type:     "study",
					Title:    title,
					Category: category,
					SubTitle: sub,
				})
			}
		}
	}

	// 5. 重要会议（meetings 表）：匹配 title, location
	meetingQuery := `SELECT id, title, COALESCE(meeting_date, ''), COALESCE(meeting_time, ''), COALESCE(location, '')
		FROM meetings
		WHERE title LIKE ? OR location LIKE ?
		ORDER BY meeting_date DESC, id DESC
		LIMIT ?`
	if rows, err := database.DB.Query(meetingQuery, kw, kw, limitPerCategory); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var title, mDate, mTime, location string
			if err := rows.Scan(&id, &title, &mDate, &mTime, &location); err == nil {
				timeStr := strings.TrimSpace(mDate + " " + mTime)
				sub := ""
				if timeStr != "" {
					sub = "时间: " + timeStr
				}
				if location != "" {
					if sub != "" {
						sub += " | 地点: " + location
					} else {
						sub = "地点: " + location
					}
				}
				meetingList = append(meetingList, GlobalSearchResultItem{
					ID:          id,
					Type:        "meeting",
					Title:       title,
					Time:        timeStr,
					MeetingTime: timeStr,
					Location:    location,
					SubTitle:    sub,
				})
			}
		}
	}

	// 合并为全部结果列表
	allResults := make([]GlobalSearchResultItem, 0, len(navList)+len(contacts)+len(incomingList)+len(studyList)+len(meetingList))
	allResults = append(allResults, navList...)
	allResults = append(allResults, contacts...)
	allResults = append(allResults, incomingList...)
	allResults = append(allResults, studyList...)
	allResults = append(allResults, meetingList...)

	if format == "grouped" {
		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"contacts":   contacts,
			"incoming":   incomingList,
			"study":      studyList,
			"meetings":   meetingList,
			"navigation": navList,
			"list":       allResults,
		})
		return
	}

	// 默认返回数组
	middleware.JSON(w, http.StatusOK, allResults)
}
