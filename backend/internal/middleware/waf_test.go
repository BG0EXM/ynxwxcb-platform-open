package middleware

import "testing"

func TestWafMatch(t *testing.T) {
	bad := []string{
		"1 union select password from users",
		"select/**/password/**/from users",
		`<script src="x.js">`,
		`<img src=x onerror=alert(1)>`,
		"javascript:alert(1)",
		"'; drop table users; --",
	}
	for _, s := range bad {
		if !wafMatch(s) {
			t.Errorf("应拦截: %s", s)
		}
	}
	good := []string{
		"请大家选择（select）合适的方案，详见附件。来自（from）县委办的通知",
		"Please select a date. " + string(make([]byte, 0)) + "The report came from the office after a very long description that keeps going and going and going and going and going and going and going and going and going and going and going and going and going and going and going and going and going and going and going and going; more from here",
		"更新(update)工作计划，设定(set)目标",
		"关于开展2026年文化宣传周活动的意见征求",
	}
	for _, s := range good {
		if wafMatch(s) {
			t.Errorf("误拦截正常内容: %s", s)
		}
	}
	if !jsonStringsMatch([]byte(`{"a":"<script>alert(1)</script>"}`)) {
		t.Error("JSON 转义的脚本应被识别")
	}
}
