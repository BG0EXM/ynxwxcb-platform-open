package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ynxwxcb-platform/internal/database"
)

func TestGlobalSearch(t *testing.T) {
	// 初始化临时测试数据库
	tmpDir, err := os.MkdirTemp("", "searchtest-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	if err := database.Init(dbPath); err != nil {
		t.Fatalf("failed to init database: %v", err)
	}

	// 插入测试数据
	_, err = database.DB.Exec(`
		INSERT INTO contacts (name, position, phone) VALUES ('张三', '宣教科科长', '13812345678');
		INSERT INTO incoming_docs (doc_no, title, from_unit) VALUES ('伊党发〔2026〕1号', '关于加强理论学习的意见', '伊犁州党委宣传部');
		INSERT INTO study_materials (title, category) VALUES ('二十届三中全会公报辅导材料', '理论学习');
		INSERT INTO meetings (title, meeting_date, meeting_time, location) VALUES ('部务会办公例会', '2026-10-10', '10:00', '三楼会议室');
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// 1. 测试空搜索词返回空数组 []
	{
		req := httptest.NewRequest("GET", "/api/global-search?q=", nil)
		rec := httptest.NewRecorder()
		GlobalSearch(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var res []GlobalSearchResultItem
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if len(res) != 0 {
			t.Fatalf("expected empty array, got %d items", len(res))
		}
	}

	// 2. 测试页面导航检索（如“收文”）
	{
		req := httptest.NewRequest("GET", "/api/global-search?q=收文", nil)
		rec := httptest.NewRecorder()
		GlobalSearch(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var res []GlobalSearchResultItem
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		foundNav := false
		for _, item := range res {
			if item.Type == "nav" && item.Path == "/incoming" {
				foundNav = true
				break
			}
		}
		if !foundNav {
			t.Fatalf("expected nav result for /incoming, got: %s", rec.Body.String())
		}
	}

	// 3. 测试通讯录检索（如“张三”）
	{
		req := httptest.NewRequest("GET", "/api/global-search?q=张三", nil)
		rec := httptest.NewRecorder()
		GlobalSearch(rec, req)

		var res []GlobalSearchResultItem
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		foundContact := false
		for _, item := range res {
			if item.Type == "contact" && item.Name == "张三" && item.Phone == "13812345678" {
				foundContact = true
				break
			}
		}
		if !foundContact {
			t.Fatalf("expected contact result for 张三, got: %s", rec.Body.String())
		}
	}

	// 4. 测试收文检索（如“理论学习”）
	{
		req := httptest.NewRequest("GET", "/api/global-search?q=理论学习", nil)
		rec := httptest.NewRecorder()
		GlobalSearch(rec, req)

		var res []GlobalSearchResultItem
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		foundDoc := false
		foundStudy := false
		for _, item := range res {
			if item.Type == "incoming" && item.Title == "关于加强理论学习的意见" {
				foundDoc = true
			}
			if item.Type == "study" && item.Title == "二十届三中全会公报辅导材料" {
				foundStudy = true
			}
		}
		if !foundDoc {
			t.Errorf("expected incoming doc matching 理论学习")
		}
		if !foundStudy {
			t.Errorf("expected study material matching 理论学习")
		}
	}

	// 5. 测试会议检索（如“部务会”）
	{
		req := httptest.NewRequest("GET", "/api/global-search?q=部务会", nil)
		rec := httptest.NewRecorder()
		GlobalSearch(rec, req)

		var res []GlobalSearchResultItem
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		foundMeeting := false
		for _, item := range res {
			if item.Type == "meeting" && item.Title == "部务会办公例会" {
				foundMeeting = true
				break
			}
		}
		if !foundMeeting {
			t.Fatalf("expected meeting result for 部务会, got: %s", rec.Body.String())
		}
	}
}
