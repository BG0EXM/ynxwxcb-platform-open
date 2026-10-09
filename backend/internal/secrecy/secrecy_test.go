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
	path := "../../data/uploads/solicits/2026/10/1791477665531967000_测试文件_01.png"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("测试文件_01.png", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for image OCR, but got not violated")
	}
	t.Logf("Success! Image OCR violation detected: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestUserScannedPdf2(t *testing.T) {
	path := "../../data/uploads/solicits/2026/10/1791477707527052000_测试文件2.pdf"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}
	defer f.Close()

	res, _ := CheckFileSecrecy("测试文件2.pdf", f)
	if !res.Violated {
		t.Fatalf("Expected secrecy violation for scanned pdf 2, but got not violated")
	}
	t.Logf("Success! Scanned PDF 2 violation detected: %v, Rule: %s, Detail: %s", res.Violated, res.Rule, res.Detail)
}

func TestExtractImagesFromPdf(t *testing.T) {
	for _, path := range []string{
		"../../data/uploads/2026/10/1791475476200555000_测试文件.pdf",
		"../../data/uploads/solicits/2026/10/1791477707527052000_测试文件2.pdf",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("File not found: %s", path)
		}
		imgs := extractImagesFromPdf(data)
		t.Logf("File: %s, Extracted images count: %d", path, len(imgs))
		if len(imgs) == 0 {
			t.Fatalf("Failed to extract any images from %s", path)
		}
		for i, imgData := range imgs {
			t.Logf("  Image %d size: %d bytes", i+1, len(imgData))
		}
	}
}
