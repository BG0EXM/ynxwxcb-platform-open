package secrecy

import (
	"os"
	"testing"
)

func TestUserUploadedDocx(t *testing.T) {
	path := "../../data/uploads/2026/10/1791475492590470000_测试文件.docx"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("测试文件.docx", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for user test docx, but got not violated")
	}
	t.Logf("Success! Violated: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestSamplePdf(t *testing.T) {
	path := "/tmp/test_jimi.pdf"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("测试正文.pdf", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for test pdf, but got not violated")
	}
	t.Logf("Success! Violated: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestNormalCleanDocument(t *testing.T) {
	path := "../../data/uploads/2026/10/1791468054844513000_0603 伊宁县国民经济和社会发展第十五个五年规划纲要（公开版）.pdf"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("0603 伊宁县国民经济和社会发展第十五个五年规划纲要（公开版）.pdf", f)
	if res.Violated {
		t.Fatalf("Unexpected violation for clean public document: %v, %s, %s", res.Violated, res.Rule, res.Detail)
	}
	t.Logf("Clean document passed! Violated: %v", res.Violated)
}

func TestDocxHeaderSecrecy(t *testing.T) {
	path := "/tmp/test_header_secret.docx"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("某普通文件.docx", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for docx with header secret, but got not violated")
	}
	t.Logf("Success! Header violation detected: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestUserImageBasedPdf(t *testing.T) {
	path := "../../data/uploads/2026/10/1791475476200555000_测试文件.pdf"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("普通汇报材料.pdf", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for user image-based test PDF, but got not violated")
	}
	t.Logf("Success! Scanned/Image PDF violation detected: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestImageOcrSecrecy(t *testing.T) {
	path := "/tmp/test_jimi.png"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("单位公函照片.png", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for image OCR, but got not violated")
	}
	t.Logf("Success! Image OCR violation detected: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}
