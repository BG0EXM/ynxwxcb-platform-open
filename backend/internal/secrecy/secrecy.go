package secrecy

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/dslipak/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// SecrecyRule 定义检测规则与严重级别
type SecrecyRule struct {
	Name     string
	Category string // "国家秘密" 或 "工作秘密/内部级"
	Keywords []string
	Regex    *regexp.Regexp
}

// 核心涉密规则库
var (
	// 国家秘密密级标识
	ruleTopSecret = SecrecyRule{
		Name:     "国家秘密密级标识(绝密)",
		Category: "国家秘密",
		Keywords: []string{"绝密", "绝密级"},
		Regex:    regexp.MustCompile(`绝密`),
	}
	ruleConfidential = SecrecyRule{
		Name:     "国家秘密密级标识(机密)",
		Category: "国家秘密",
		Keywords: []string{"机密", "机密级"},
		Regex:    regexp.MustCompile(`机密`),
	}
	ruleSecretStar = SecrecyRule{
		Name:     "国家秘密密级及保密期限标识(★)",
		Category: "国家秘密",
		Keywords: []string{"★", "秘密★", "机密★", "绝密★"},
		Regex:    regexp.MustCompile(`(绝密|机密|秘密|内部|商业秘密|工作秘密)\s*[★☆﹡＊\*]|\d+\s*年\s*[★☆﹡＊\*]|[★☆﹡＊\*]\s*\d+\s*(年|个月|天)|[★☆﹡＊\*]\s*长期`),
	}
	ruleSecret = SecrecyRule{
		Name:     "国家秘密密级标识(秘密)",
		Category: "国家秘密",
		Keywords: []string{"秘密级", "密级：秘密", "密级:秘密", "属于国家秘密", "国家秘密"},
		Regex:    regexp.MustCompile(`(秘密级|密级\s*[:：]\s*秘密|国家秘密|保守国家秘密法|定密依据|保密期限\s*[:：])`),
	}

	// 工作秘密与内部级标识
	ruleWorkSecret = SecrecyRule{
		Name:     "工作秘密标识",
		Category: "工作秘密",
		Keywords: []string{"工作秘密", "工作秘密★"},
		Regex:    regexp.MustCompile(`工作秘密`),
	}
	ruleInternal = SecrecyRule{
		Name:     "内部资料与敏感控制标识",
		Category: "内部级资料",
		Keywords: []string{"内部资料", "内部文件", "内部掌握", "内部参考", "内部使用", "不得外传", "严禁外传", "严禁外发", "非公开发布"},
		Regex:    regexp.MustCompile(`(内部资料|内部文件|内部掌握|内部参考|内部使用|不得外传|严禁外传|严禁外发|非公开发布)`),
	}

	// 规则集合
	allRules = []SecrecyRule{
		ruleTopSecret,
		ruleConfidential,
		ruleSecretStar,
		ruleWorkSecret,
		ruleInternal,
		ruleSecret,
	}

	xmlTagRegex  = regexp.MustCompile(`<[^>]+>`)
	pdfHexRegex  = regexp.MustCompile(`<([0-9a-fA-F]+)>`)
	pdfBfChar    = regexp.MustCompile(`<([0-9a-fA-F]+)>\s*<([0-9a-fA-F]+)>`)
	pdfBfRange   = regexp.MustCompile(`<([0-9a-fA-F]+)>\s*<([0-9a-fA-F]+)>\s*<([0-9a-fA-F]+)>`)
	pdfStreamRe  = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
)

// ViolationResult 涉密检测结果
type ViolationResult struct {
	Violated bool   // 是否命中
	Rule     string // 命中的规则名称
	Category string // 涉密类别（国家秘密/工作秘密/内部级）
	Detail   string // 命中上下文详情
}

// CheckFileSecrecy 深度检测文件是否包含国家秘密或内部保密标识
// 覆盖：文件名、Word(.docx/.doc)、PDF(.pdf)、Excel(.xlsx)、纯文本内容
func CheckFileSecrecy(filename string, r io.Reader) (ViolationResult, io.Reader) {
	// 1. 文件名检测
	if res := checkFilename(filename); res.Violated {
		return res, r
	}

	if r == nil {
		return ViolationResult{Violated: false}, r
	}

	// 2. 读取全部字节（最大限制 30MB 深度检测，防止 OOM）
	maxRead := int64(30 << 20)
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, r, maxRead)
	if err != nil && err != io.EOF {
		// 读取错误返回原 reader
		return ViolationResult{Violated: false}, io.MultiReader(bytes.NewReader(buf.Bytes()), r)
	}

	fileBytes := buf.Bytes()
	restoredReader := io.MultiReader(bytes.NewReader(fileBytes), r)

	if n == 0 {
		return ViolationResult{Violated: false}, restoredReader
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// 3. 针对不同文件类型进行深度内容解构与检测
	switch ext {
	case ".docx":
		if res := checkDocx(fileBytes); res.Violated {
			return res, restoredReader
		}
	case ".doc":
		if res := checkDocLegacy(fileBytes); res.Violated {
			return res, restoredReader
		}
	case ".pdf":
		// 1. 深度分析 PDF 内部文本流、CMap 与字库编码
		if res := checkPdf(fileBytes); res.Violated {
			return res, restoredReader
		}
		// 2. 深度视觉 OCR 识别（涵盖扫描型/图片型 PDF 及含公章扫描件）
		if ocrText, err := runOCROnBytes(".pdf", fileBytes); err == nil && ocrText != "" {
			if res := matchTextRules(ocrText, "PDF扫描/图像公文OCR识别"); res.Violated {
				return res, restoredReader
			}
		}
	case ".jpg", ".jpeg", ".png", ".bmp", ".webp", ".tif", ".tiff":
		// 3. 各单位公函照片、盖章件与红头扫描图片 OCR 智能识别
		if ocrText, err := runOCROnBytes(ext, fileBytes); err == nil && ocrText != "" {
			if res := matchTextRules(ocrText, "公文图片附件OCR智能识别"); res.Violated {
				return res, restoredReader
			}
		}
	case ".xlsx":
		if res := checkXlsx(fileBytes); res.Violated {
			return res, restoredReader
		}
	case ".txt", ".md", ".json", ".xml", ".html", ".csv":
		if res := checkPlainText(string(fileBytes)); res.Violated {
			return res, restoredReader
		}
	}

	// 4. 通用二进制/编码嗅探兜底（即使后缀名被伪造或特殊混淆）
	if res := checkRawBytesFallback(fileBytes); res.Violated {
		return res, restoredReader
	}

	return ViolationResult{Violated: false}, restoredReader
}

// checkFilename 检查文件名
func checkFilename(filename string) ViolationResult {
	cleanName := strings.ReplaceAll(filename, " ", "")
	cleanName = strings.ReplaceAll(cleanName, "_", "")
	cleanName = strings.ReplaceAll(cleanName, "-", "")

	for _, rule := range allRules {
		for _, kw := range rule.Keywords {
			if strings.Contains(cleanName, kw) {
				return ViolationResult{
					Violated: true,
					Rule:     rule.Name,
					Category: rule.Category,
					Detail:   fmt.Sprintf("文件名中命中关键字: %s", kw),
				}
			}
		}
		if rule.Regex != nil && rule.Regex.MatchString(cleanName) {
			return ViolationResult{
				Violated: true,
				Rule:     rule.Name,
				Category: rule.Category,
				Detail:   fmt.Sprintf("文件名命中特征规则: %s", rule.Name),
			}
		}
	}
	return ViolationResult{Violated: false}
}

// checkDocx 解析 Word .docx 文件（解压 word/document.xml, word/header*.xml, docProps/*.xml）
func checkDocx(data []byte) ViolationResult {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ViolationResult{Violated: false}
	}

	var sb strings.Builder
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		// 重点扫描正文、页眉（密级常标于版头页眉）、页脚、核心属性
		if strings.HasPrefix(name, "word/") && (strings.HasSuffix(name, ".xml")) ||
			strings.HasPrefix(name, "docprops/") && strings.HasSuffix(name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			// 清洗 XML 标签，提取出纯文本
			plain := xmlTagRegex.ReplaceAllString(string(content), " ")
			sb.WriteString(plain)
			sb.WriteString("\n")
		}
	}

	fullText := sb.String()
	return matchTextRules(fullText, "Word文档正文/页眉内容")
}

// checkDocLegacy 解析 Word 97-2003 .doc 文件（UTF-16LE 文本与原始文本检测）
func checkDocLegacy(data []byte) ViolationResult {
	// 1. 尝试将连续的 UTF-16LE 字节转为 UTF-8 字符串
	var runes []rune
	for i := 0; i+1 < len(data); i += 2 {
		val := uint16(data[i]) | (uint16(data[i+1]) << 8)
		if val >= 0x20 && val != 0xFFFF {
			runes = append(runes, rune(val))
		} else if len(runes) > 0 && runes[len(runes)-1] != ' ' {
			runes = append(runes, ' ')
		}
	}
	docText := string(runes)
	if res := matchTextRules(docText, "Word(DOC)正文内容"); res.Violated {
		return res
	}

	// 2. 编码特征搜索（直接在原始二进制匹配 UTF-16LE 关键字字节序列）
	return matchEncodedBytes(data, "Word(DOC)文档字节流")
}

// checkXlsx 解析 Excel .xlsx 文件
func checkXlsx(data []byte) ViolationResult {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ViolationResult{Violated: false}
	}

	var sb strings.Builder
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if name == "xl/sharedstrings.xml" || strings.HasPrefix(name, "xl/worksheets/") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			plain := xmlTagRegex.ReplaceAllString(string(content), " ")
			sb.WriteString(plain)
			sb.WriteString("\n")
		}
	}
	return matchTextRules(sb.String(), "Excel表格文本内容")
}

// checkPdf 深度解构与分析 PDF 文件
func checkPdf(data []byte) ViolationResult {
	// 策略 1: 使用高层 PDF 文本提取引擎 (dslipak/pdf)
	if res := extractPdfWithHighLevelReader(data); res.Violated {
		return res
	}

	// 策略 2: PDF Stream 深度解压与 CMap / 编码分析
	if res := scanPdfStreams(data); res.Violated {
		return res
	}

	// 策略 3: PDF 元数据与原始文本匹配
	return matchEncodedBytes(data, "PDF文档字节流")
}

// extractPdfWithHighLevelReader 使用 PDF 阅读器提取正文
func extractPdfWithHighLevelReader(data []byte) ViolationResult {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ViolationResult{Violated: false}
	}

	plainReader, err := r.GetPlainText()
	if err != nil {
		return ViolationResult{Violated: false}
	}

	extracted, err := io.ReadAll(plainReader)
	if err == nil && len(extracted) > 0 {
		text := string(extracted)
		if res := matchTextRules(text, "PDF提取文本内容"); res.Violated {
			return res
		}
	}
	return ViolationResult{Violated: false}
}

// scanPdfStreams 解压 PDF 内部 FlateDecode Stream 并进行多编码比对与 CMap 还原
func scanPdfStreams(data []byte) ViolationResult {
	matches := pdfStreamRe.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return ViolationResult{Violated: false}
	}

	// 收集并解析 ToUnicode CMap
	cmap := make(map[string]rune)
	var decompressedStreams [][]byte

	for _, m := range matches {
		streamBytes := m[1]
		// 尝试 zlib 解压
		zr, err := zlib.NewReader(bytes.NewReader(streamBytes))
		var decomp []byte
		if err == nil {
			decomp, _ = io.ReadAll(zr)
			zr.Close()
		} else {
			decomp = streamBytes
		}

		if len(decomp) > 0 {
			decompressedStreams = append(decompressedStreams, decomp)

			// 检查是否包含 ToUnicode CMap 定义
			if bytes.Contains(decomp, []byte("beginbfchar")) || bytes.Contains(decomp, []byte("beginbfrange")) {
				parseCMap(decomp, cmap)
			}
		}
	}

	var sb strings.Builder
	for _, decomp := range decompressedStreams {
		// 1. 尝试直接按 UTF-8 / GBK 扫描文本流
		strUTF8 := string(decomp)
		sb.WriteString(strUTF8)
		sb.WriteString("\n")

		// GBK 解码尝试
		gbkReader := simplifiedchinese.GBK.NewDecoder().Reader(bytes.NewReader(decomp))
		if gbkBytes, err := io.ReadAll(gbkReader); err == nil {
			sb.WriteString(string(gbkBytes))
			sb.WriteString("\n")
		}

		// 2. 检查 UTF-16BE / UTF-16LE 字节
		if res := matchEncodedBytes(decomp, "PDF解压数据流"); res.Violated {
			return res
		}

		// 3. 如果提取到了 CMap，对十六进制 Tj 操作符进行还原: <07A2>Tj 或 [(<...>) ...]
		if len(cmap) > 0 {
			hexMatches := pdfHexRegex.FindAllSubmatch(decomp, -1)
			var cmapText []rune
			for _, hm := range hexMatches {
				hexStr := strings.ToUpper(string(hm[1]))
				for i := 0; i+4 <= len(hexStr); i += 4 {
					code := hexStr[i : i+4]
					if ch, ok := cmap[code]; ok {
						cmapText = append(cmapText, ch)
					}
				}
			}
			if len(cmapText) > 0 {
				sb.WriteString(string(cmapText))
				sb.WriteString("\n")
			}
		}
	}

	fullStreamText := sb.String()
	return matchTextRules(fullStreamText, "PDF解压内容流")
}

// parseCMap 解析 PDF 中的 ToUnicode CMap
func parseCMap(decomp []byte, cmap map[string]rune) {
	// 匹配 beginbfchar
	for _, m := range pdfBfChar.FindAllSubmatch(decomp, -1) {
		src := strings.ToUpper(string(m[1]))
		dst := strings.ToUpper(string(m[2]))
		if len(dst) == 4 {
			if val, err := strconv.ParseInt(dst, 16, 32); err == nil {
				cmap[src] = rune(val)
			}
		}
	}
	// 匹配 beginbfrange
	for _, m := range pdfBfRange.FindAllSubmatch(decomp, -1) {
		s1, err1 := strconv.ParseInt(string(m[1]), 16, 32)
		s2, err2 := strconv.ParseInt(string(m[2]), 16, 32)
		d1, err3 := strconv.ParseInt(string(m[3]), 16, 32)
		if err1 == nil && err2 == nil && err3 == nil && s2 >= s1 && s2-s1 < 5000 {
			for code := s1; code <= s2; code++ {
				key := fmt.Sprintf("%04X", code)
				cmap[key] = rune(d1 + (code - s1))
			}
		}
	}
}

// checkPlainText 检查纯文本
func checkPlainText(text string) ViolationResult {
	return matchTextRules(text, "文件正文内容")
}

// matchTextRules 统一对文本内容执行规则正则和关键字匹配
func matchTextRules(text, source string) ViolationResult {
	if text == "" {
		return ViolationResult{Violated: false}
	}

	// 去除多余空白和换行以防通过空格隐藏
	cleanText := strings.ReplaceAll(text, "\t", "")
	cleanText = strings.ReplaceAll(cleanText, "\r", "")
	compactText := strings.ReplaceAll(cleanText, " ", "")

	for _, rule := range allRules {
		// 1. 正则优先匹配（如 "秘密★5年"、"绝密★长期"）
		if rule.Regex != nil {
			if loc := rule.Regex.FindStringIndex(compactText); loc != nil {
				matched := compactText[loc[0]:loc[1]]
				return ViolationResult{
					Violated: true,
					Rule:     rule.Name,
					Category: rule.Category,
					Detail:   fmt.Sprintf("%s中检测到高危标识:「%s」", source, matched),
				}
			}
		}

		// 2. 关键字匹配
		for _, kw := range rule.Keywords {
			if strings.Contains(compactText, kw) {
				return ViolationResult{
					Violated: true,
					Rule:     rule.Name,
					Category: rule.Category,
					Detail:   fmt.Sprintf("%s中检测到涉密词汇:「%s」", source, kw),
				}
			}
		}
	}

	return ViolationResult{Violated: false}
}

// checkRawBytesFallback 兜底直接在原始字节流中寻找常见编码的涉密特征
func checkRawBytesFallback(data []byte) ViolationResult {
	return matchEncodedBytes(data, "文件原始数据流")
}

// matchEncodedBytes 对字节数组匹配 UTF-8, UTF-16LE, UTF-16BE 和 Hex 字符串
func matchEncodedBytes(data []byte, source string) ViolationResult {
	// 重点检查的涉密词
	keyWords := []string{
		"机密", "绝密", "秘密", "工作秘密", "内部资料", "内部文件", "不得外传", "严禁外传",
	}

	for _, kw := range keyWords {
		// 1. UTF-8
		bUTF8 := []byte(kw)
		if bytes.Contains(data, bUTF8) {
			cat := "国家秘密"
			if strings.Contains(kw, "内部") || strings.Contains(kw, "工作") || strings.Contains(kw, "外传") {
				cat = "工作秘密/内部级"
			}
			return ViolationResult{
				Violated: true,
				Rule:     fmt.Sprintf("深度扫描命中密级标识(%s)", kw),
				Category: cat,
				Detail:   fmt.Sprintf("%s深度检测命中涉密特征:「%s」", source, kw),
			}
		}

		// 2. UTF-16LE
		bUTF16LE := encodeUTF16LE(kw)
		if bytes.Contains(data, bUTF16LE) {
			cat := "国家秘密"
			if strings.Contains(kw, "内部") || strings.Contains(kw, "工作") || strings.Contains(kw, "外传") {
				cat = "工作秘密/内部级"
			}
			return ViolationResult{
				Violated: true,
				Rule:     fmt.Sprintf("深度扫描命中UTF-16LE密级标识(%s)", kw),
				Category: cat,
				Detail:   fmt.Sprintf("%s编码特征命中涉密标识:「%s」", source, kw),
			}
		}

		// 3. UTF-16BE (PDF 常用)
		bUTF16BE := encodeUTF16BE(kw)
		if bytes.Contains(data, bUTF16BE) {
			cat := "国家秘密"
			if strings.Contains(kw, "内部") || strings.Contains(kw, "工作") || strings.Contains(kw, "外传") {
				cat = "工作秘密/内部级"
			}
			return ViolationResult{
				Violated: true,
				Rule:     fmt.Sprintf("深度扫描命中UTF-16BE密级标识(%s)", kw),
				Category: cat,
				Detail:   fmt.Sprintf("%s编码特征命中涉密标识:「%s」", source, kw),
			}
		}

		// 4. Hex string (如 <673A5BC6> 代表 机密)
		hexUpper := strings.ToUpper(fmt.Sprintf("%X", bUTF16BE))
		if bytes.Contains(data, []byte(hexUpper)) || bytes.Contains(data, []byte(strings.ToLower(hexUpper))) {
			cat := "国家秘密"
			if strings.Contains(kw, "内部") || strings.Contains(kw, "工作") || strings.Contains(kw, "外传") {
				cat = "工作秘密/内部级"
			}
			return ViolationResult{
				Violated: true,
				Rule:     fmt.Sprintf("深度扫描命中十六进制密级代码(%s)", kw),
				Category: cat,
				Detail:   fmt.Sprintf("%s代码特征命中涉密标识:「%s」", source, kw),
			}
		}
	}

	return ViolationResult{Violated: false}
}

func encodeUTF16LE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	b := make([]byte, len(u16)*2)
	for i, val := range u16 {
		b[i*2] = byte(val)
		b[i*2+1] = byte(val >> 8)
	}
	return b
}

func encodeUTF16BE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	b := make([]byte, len(u16)*2)
	for i, val := range u16 {
		b[i*2] = byte(val >> 8)
		b[i*2+1] = byte(val)
	}
	return b
}
