package secrecy

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	tessOnce        sync.Once
	tessBinPath     string
	tessDataDir     string
	tessLangs       []string
	tessInitialized bool
	pdftoppmPath    string

	pdfImageRe = regexp.MustCompile(`(?s)<<([^>]*?/Subtype\s*/Image[^>]*)>>\s*stream\r?\n(.*?)\r?\nendstream`)
	widthRe    = regexp.MustCompile(`/Width\s+(\d+)`)
	heightRe   = regexp.MustCompile(`/Height\s+(\d+)`)
	colorSpRe  = regexp.MustCompile(`/ColorSpace\s*/([A-Za-z0-9_]+)`)
	bpcRe      = regexp.MustCompile(`/BitsPerComponent\s+(\d+)`)
)

// InitOCR 启动自检并探测系统的 OCR 与 PDF 视觉渲染引擎配置
func InitOCR() {
	tessOnce.Do(func() {
		// 1. 探测 Tesseract 可执行文件
		tessBinPath = findTesseractPath()
		if tessBinPath == "" {
			log.Printf("[OCR] ⚠️ 未在系统检测到 tesseract。若需对公文图片及扫描版PDF进行涉密审查，请在服务器执行: apt install -y tesseract-ocr tesseract-ocr-chi-sim (Ubuntu/Debian) 或 yum install -y tesseract tesseract-langpack-chi_sim (CentOS/RHEL)")
		} else {
			tessDataDir = findTessdataDir()

			// 获取可用语言列表
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, tessBinPath, "--list-langs")
			if tessDataDir != "" {
				cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessDataDir)
			}
			out, err := cmd.CombinedOutput()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l != "" && !strings.HasPrefix(l, "List of") {
						tessLangs = append(tessLangs, l)
					}
				}
			}

			tessInitialized = true
			hasChinese := hasLanguage("chi_sim") || hasLanguage("chi-sim")
			if hasChinese {
				log.Printf("[OCR] ✅ Tesseract OCR 引擎就绪: 路径=%s, 语言字库=%v, tessdata=%s", tessBinPath, tessLangs, tessDataDir)
			} else {
				log.Printf("[OCR] ⚠️ 警告：检测到 Tesseract (%s)，但未安装中文简体字库(chi_sim)！当前可用语言: %v。请在服务器执行: apt install -y tesseract-ocr-chi-sim (Ubuntu/Debian) 或 yum install -y tesseract-langpack-chi_sim (CentOS/RHEL)", tessBinPath, tessLangs)
			}
		}

		// 2. 探测 pdftoppm 渲染工具
		pdftoppmPath = findPdftoppmPath()
		if pdftoppmPath != "" {
			log.Printf("[OCR] ✅ 检测到 PDF 高清栅格化工具: %s (支持 150 DPI 公文整页视觉审查)", pdftoppmPath)
		} else {
			log.Printf("[OCR] ℹ️ 未安装 pdftoppm (poppler-utils)，将使用纯 Go 内置引擎直接解构提取 PDF 扫描图像 (如需整页渲染支持建议: apt install -y poppler-utils)")
		}
	})
}

// findTesseractPath 探测系统中的 tesseract 可执行文件路径
func findTesseractPath() string {
	if p, err := exec.LookPath("tesseract"); err == nil {
		return p
	}
	candidates := []string{
		"/usr/bin/tesseract",
		"/usr/local/bin/tesseract",
		"/opt/homebrew/bin/tesseract",
		"/usr/local/Cellar/tesseract",
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// findPdftoppmPath 探测系统中的 pdftoppm 可执行文件路径
func findPdftoppmPath() string {
	if p, err := exec.LookPath("pdftoppm"); err == nil {
		return p
	}
	candidates := []string{
		"/usr/bin/pdftoppm",
		"/usr/local/bin/pdftoppm",
		"/opt/homebrew/bin/pdftoppm",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// findTessdataDir 探测 Linux/macOS 常见 tessdata 字典目录
func findTessdataDir() string {
	if env := os.Getenv("TESSDATA_PREFIX"); env != "" {
		if fi, err := os.Stat(env); err == nil && fi.IsDir() {
			return env
		}
	}
	candidates := []string{
		"/usr/share/tesseract-ocr/4.00/tessdata",
		"/usr/share/tesseract-ocr/5/tessdata",
		"/usr/share/tesseract-ocr/tessdata",
		"/usr/share/tessdata",
		"/usr/local/share/tessdata",
		"/opt/homebrew/share/tessdata",
		`C:\Program Files\Tesseract-OCR\tessdata`,
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

func hasLanguage(lang string) bool {
	for _, l := range tessLangs {
		if strings.EqualFold(l, lang) {
			return true
		}
	}
	return false
}

// buildLangsArg 根据实际安装的模型构建最匹配的语言参数
func buildLangsArg() string {
	// 优先简体中文（党政机关公文标准规范），绝不主动混入 chi_tra
	// 一旦将 chi_sim 与 chi_tra 混合传参，Tesseract 遇到简体字时会因笔画权重优先输出繁体字（如「機密」）
	if hasLanguage("chi_sim") {
		if hasLanguage("eng") {
			return "chi_sim+eng"
		}
		return "chi_sim"
	} else if hasLanguage("chi-sim") {
		if hasLanguage("eng") {
			return "chi-sim+eng"
		}
		return "chi-sim"
	}

	// 仅当服务器未安装简体字库时，才回退繁体字库
	if hasLanguage("chi_tra") {
		if hasLanguage("eng") {
			return "chi_tra+eng"
		}
		return "chi_tra"
	}

	if hasLanguage("eng") {
		return "eng"
	}

	return ""
}

// runTesseractOnFile 对单个图片文件运行 Tesseract
func runTesseractOnFile(filePath string) (string, error) {
	InitOCR()
	if tessBinPath == "" {
		return "", fmt.Errorf("tesseract 未安装")
	}

	langArg := buildLangsArg()
	start := time.Now()

	var psm3Text, psm11Text string

	// 方案1：标准自动页面分割模式 (--psm 3)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel1()

	args := []string{filePath, "stdout"}
	if tessDataDir != "" {
		args = append(args, "--tessdata-dir", tessDataDir)
	}
	if langArg != "" {
		args = append(args, "-l", langArg)
	}
	args = append(args, "--psm", "3")

	cmd := exec.CommandContext(ctx1, tessBinPath, args...)
	if tessDataDir != "" {
		cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessDataDir)
	}

	out, err := cmd.CombinedOutput()
	if err == nil {
		psm3Text = strings.TrimSpace(string(out))
	} else {
		log.Printf("[OCR] Tesseract(psm 3) 执行异常: %v, 输出: %s", err, string(out))
	}

	// 方案2：稀疏模式 (--psm 11)，对公文红头角落密级/印章特别有效
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	argsSparse := []string{filePath, "stdout"}
	if tessDataDir != "" {
		argsSparse = append(argsSparse, "--tessdata-dir", tessDataDir)
	}
	if langArg != "" {
		argsSparse = append(argsSparse, "-l", langArg)
	}
	argsSparse = append(argsSparse, "--psm", "11")

	cmdSparse := exec.CommandContext(ctx2, tessBinPath, argsSparse...)
	if tessDataDir != "" {
		cmdSparse.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessDataDir)
	}

	outSparse, errSparse := cmdSparse.CombinedOutput()
	if errSparse == nil {
		psm11Text = strings.TrimSpace(string(outSparse))
	}

	// 智能合并识别结果
	var merged string
	if psm3Text != "" && psm11Text != "" {
		if strings.Contains(psm3Text, psm11Text) {
			merged = psm3Text
		} else if strings.Contains(psm11Text, psm3Text) {
			merged = psm11Text
		} else {
			merged = psm3Text + "\n" + psm11Text
		}
	} else if psm3Text != "" {
		merged = psm3Text
	} else {
		merged = psm11Text
	}

	cost := time.Since(start)
	if len(merged) > 0 {
		preview := merged
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		log.Printf("[OCR] Tesseract 识别完成: 耗时=%v, 字符数=%d, 预览=[%s]", cost, len(merged), preview)
	} else {
		log.Printf("[OCR] Tesseract 识别完成: 耗时=%v, 未识别出有效文本", cost)
	}

	return merged, nil
}

// runNativeAppleVision 尝试调用 macOS 原生 Vision 引擎（若存在）
func runNativeAppleVision(filePath string) string {
	candidates := []string{
		"bin/doc_vision_ocr",
		"backend/bin/doc_vision_ocr",
		"/tmp/doc_vision_ocr",
	}
	if execPath, err := os.Executable(); err == nil {
		dir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(dir, "bin", "doc_vision_ocr"),
			filepath.Join(dir, "doc_vision_ocr"),
		)
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, c, filePath)
			if out, err := cmd.Output(); err == nil && len(out) > 0 {
				return string(out)
			}
			break
		}
	}
	return ""
}

// runOCROnImage 执行普通图片文件的 OCR 识别
func runOCROnImage(ext string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	log.Printf("[OCR] 正在执行公文图片 OCR 识别: 格式=%s, 大小=%d 字节", ext, len(data))

	tmpFile, err := os.CreateTemp("", "secrecy_img_*"+ext)
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// 1. 如果在 macOS 下且存在原生工具，优先使用
	if txt := runNativeAppleVision(tmpPath); txt != "" {
		log.Printf("[OCR] macOS Apple Vision 原生引擎识别完成: 字符数=%d", len(txt))
		return txt, nil
	}

	// 2. 调用跨平台通用 Tesseract OCR
	return runTesseractOnFile(tmpPath)
}

// runOCROnPdf 执行 PDF 文档的图像化 OCR 识别（重点支持纯图片型/扫描型 PDF）
func runOCROnPdf(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	log.Printf("[OCR] 正在执行 PDF 公文视觉识别: 大小=%d 字节", len(data))

	tmpPdf, err := os.CreateTemp("", "secrecy_pdf_*.pdf")
	if err != nil {
		return "", err
	}
	pdfPath := tmpPdf.Name()
	defer os.Remove(pdfPath)

	if _, err := tmpPdf.Write(data); err != nil {
		tmpPdf.Close()
		return "", err
	}
	tmpPdf.Close()

	var sb strings.Builder

	// 1. 优先尝试 macOS 原生 Vision（若在 Mac 上运行）
	if txt := runNativeAppleVision(pdfPath); txt != "" {
		log.Printf("[OCR] macOS Apple Vision 原生引擎识别 PDF 完成: 字符数=%d", len(txt))
		sb.WriteString(txt)
		sb.WriteString("\n")
	}

	// 2. 尝试系统级 pdftoppm 渲染公文前 3 页为 PNG 图片（Linux poppler-utils 标准工具）
	if renderedPages := renderPdfPagesWithPdftoppm(pdfPath); len(renderedPages) > 0 {
		log.Printf("[OCR] 系统 pdftoppm 成功渲染 PDF 页面: %d 页", len(renderedPages))
		for _, pageImg := range renderedPages {
			defer os.Remove(pageImg)
			if pageTxt, err := runTesseractOnFile(pageImg); err == nil && pageTxt != "" {
				sb.WriteString(pageTxt)
				sb.WriteString("\n")
			}
		}
	} else {
		// 3. 纯 Go 原生解包：直接从 PDF 数据流中提取扫描仪嵌入的原始图片（零系统依赖兜底！）
		extractedImages := extractImagesFromPdf(data)
		if len(extractedImages) > 0 {
			log.Printf("[OCR] 纯 Go 原生解包成功从 PDF 提取出 %d 张内嵌扫描图片", len(extractedImages))
			for i, imgBytes := range extractedImages {
				if i >= 3 {
					break // 重点审查前 3 页，防恶意超大文件消耗算力
				}
				tmpImg, err := os.CreateTemp("", "secrecy_extracted_*.png")
				if err != nil {
					continue
				}
				imgPath := tmpImg.Name()
				tmpImg.Write(imgBytes)
				tmpImg.Close()

				if txt, err := runTesseractOnFile(imgPath); err == nil && txt != "" {
					sb.WriteString(txt)
					sb.WriteString("\n")
				}
				os.Remove(imgPath)
			}
		} else {
			log.Printf("[OCR] PDF 未提取到内嵌图片，且未安装 pdftoppm")
		}
	}

	res := sb.String()
	log.Printf("[OCR] PDF 综合识别汇总字符数: %d", len(res))
	return res, nil
}

// renderPdfPagesWithPdftoppm 使用 pdftoppm 将 PDF 前 1~3 页转为图片
func renderPdfPagesWithPdftoppm(pdfPath string) []string {
	InitOCR()
	if pdftoppmPath == "" {
		return nil
	}

	prefix := pdfPath + "_page"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 提取前 1~3 页，分辨率 150 DPI（既保证 OCR 清晰度，又防止图片过大）
	cmd := exec.CommandContext(ctx, pdftoppmPath, "-png", "-r", "150", "-f", "1", "-l", "3", pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[OCR] pdftoppm 渲染执行异常: %v, 输出: %s", err, string(out))
		return nil
	}

	matches, _ := filepath.Glob(prefix + "*.png")
	return matches
}

// extractImagesFromPdf 纯 Go 遍历解构 PDF Stream，提取内嵌的扫描图片（零三方依赖）
func extractImagesFromPdf(data []byte) [][]byte {
	matches := pdfImageRe.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil
	}

	var results [][]byte
	for _, m := range matches {
		dict := string(m[1])
		stream := m[2]

		// 提取宽高
		wMatch := widthRe.FindStringSubmatch(dict)
		hMatch := heightRe.FindStringSubmatch(dict)
		if len(wMatch) < 2 || len(hMatch) < 2 {
			continue
		}
		width, _ := strconv.Atoi(wMatch[1])
		height, _ := strconv.Atoi(hMatch[1])
		if width <= 50 || height <= 50 {
			// 过滤小图标/微型线条
			continue
		}

		// 1. DCTDecode: 标准原生 JPEG 图像（扫描仪、手机拍照公文扫描最常见格式）
		if bytes.HasPrefix(stream, []byte("\xFF\xD8\xFF")) || strings.Contains(dict, "DCTDecode") {
			results = append(results, stream)
			continue
		}

		// 2. FlateDecode: 纯像素流（支持 RGB 和 灰度位图）
		if strings.Contains(dict, "FlateDecode") {
			zr, err := zlib.NewReader(bytes.NewReader(stream))
			if err != nil {
				continue
			}
			raw, _ := io.ReadAll(zr)
			zr.Close()

			colorSpace := "DeviceRGB"
			if csMatch := colorSpRe.FindStringSubmatch(dict); len(csMatch) >= 2 {
				colorSpace = csMatch[1]
			}

			// RGB 模式 (3 字节/像素)
			if colorSpace == "DeviceRGB" && len(raw) >= width*height*3 {
				img := image.NewRGBA(image.Rect(0, 0, width, height))
				idx := 0
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						r := raw[idx]
						g := raw[idx+1]
						b := raw[idx+2]
						img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
						idx += 3
					}
				}
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err == nil {
					results = append(results, buf.Bytes())
				}
				continue
			}

			// 灰度模式 (1 字节/像素)
			if (colorSpace == "DeviceGray" || len(raw) >= width*height) && len(raw) >= width*height {
				img := image.NewGray(image.Rect(0, 0, width, height))
				idx := 0
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						img.Set(x, y, color.Gray{Y: raw[idx]})
						idx++
					}
				}
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err == nil {
					results = append(results, buf.Bytes())
				}
				continue
			}
		}
	}

	return results
}

// runOCROnBytes 将内存数据路由至对应的图片或 PDF OCR 处理器
func runOCROnBytes(ext string, data []byte) (string, error) {
	if strings.EqualFold(ext, ".pdf") {
		return runOCROnPdf(data)
	}
	return runOCROnImage(ext, data)
}
