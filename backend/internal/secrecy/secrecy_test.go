package secrecy

import (
	"bytes"
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

func TestTraditionalChineseMatch(t *testing.T) {
	for _, text := range []string{
		"测试用例 機密  測試用例",
		"機密  測試用例",
		"單位公函 絕密★1年",
		"內部資料 嚴禁外傳",
	} {
		res := matchTextRules(text, "繁体OCR测试")
		if !res.Violated {
			t.Fatalf("Expected violation for text %q, but got not violated", text)
		}
		t.Logf("Passed: %s -> %s (%s)", text, res.Rule, res.Category)
	}
}

// 保密法学习类文件名不得误判；含真实密级标识的仍须拦截
func TestCheckFilename_StudyWhitelist(t *testing.T) {
	for _, ok := range []string{"保守国家秘密法学习.pdf", "2026年保密教育培训课件.pptx", "保密知识测试题.docx"} {
		if res := checkFilename(ok); res.Violated {
			t.Errorf("学习类文件名被误判: %s (%s)", ok, res.Detail)
		}
	}
	for _, bad := range []string{"保守国家秘密法学习（机密★）.pdf", "关于XX的工作秘密.docx", "秘密★1年 通知.pdf", "内部资料汇编.pdf"} {
		if res := checkFilename(bad); !res.Violated {
			t.Errorf("含密级标识的文件名应被拦截: %s", bad)
		}
	}
}

// 损坏/无法解析的 Word 文档必须拒绝；压缩格式不做原始字节兜底匹配
func TestCheckFileSecrecy_UnreadableAndCompressed(t *testing.T) {
	res, _ := CheckFileSecrecy("报告.docx", bytes.NewReader([]byte("not a zip file at all")))
	if !res.Unreadable {
		t.Fatal("无法解析的 docx 应标记为无法检测并拒绝")
	}
	// 伪造的 zip 原始字节中恰好含有“秘密”的 UTF-16 编码，不应误判
	data := append([]byte("PK\x03\x04"), []byte{0xd8, 0x79, 0xc6, 0x5b}...)
	res, _ = CheckFileSecrecy("照片.zip", bytes.NewReader(data))
	if res.Violated {
		t.Fatalf("压缩包原始字节不应触发兜底误判: %+v", res)
	}
}
