package secrecy

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
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
	// 中文涉密公文场景：优先组合简体与繁体字典（chi_sim+chi_tra），提供最强汉字笔画互补
	// 坚决不混入英文 eng（混入 eng 会导致大篇幅白边下的汉字笔画被英文模型抢占强行识别为大写字母 LE）
	hasSim := hasLanguage("chi_sim") || hasLanguage("chi-sim")
	simName := "chi_sim"
	if !hasLanguage("chi_sim") && hasLanguage("chi-sim") {
		simName = "chi-sim"
	}
	hasTra := hasLanguage("chi_tra")

	if hasSim && hasTra {
		return simName + "+chi_tra"
	} else if hasSim {
		return simName
	} else if hasTra {
		return "chi_tra"
	}

	if hasLanguage("eng") {
		return "eng"
	}

	return ""
}

// normalizeChinese 将 OCR 吐出的繁体汉字统一规整为规范简体汉字
func normalizeChinese(s string) string {
	replacements := []string{
		"機密", "机密",
		"絕密", "绝密",
		"祕密", "秘密",
		"內部", "内部",
		"資料", "资料",
		"檔案", "档案",
		"單位", "单位",
		"測試", "测试",
		"工作祕密", "工作秘密",
		"嚴禁外傳", "严禁外传",
		"非公開發布", "非公开发布",
		"商業祕密", "商业秘密",
	}
	r := strings.NewReplacer(replacements...)
	return r.Replace(s)
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
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
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

	// 方案2：稀疏模式 (--psm 11)，仅当 psm 3 为空时作为后备重试
	if psm3Text == "" {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
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

	// 统一繁简转换为规范简体
	merged = normalizeChinese(merged)

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

	var sb strings.Builder

	// 1. 如果在 macOS 下且存在原生工具，优先使用
	if txt := runNativeAppleVision(tmpPath); txt != "" {
		log.Printf("[OCR] macOS Apple Vision 原生引擎识别完成: 字符数=%d", len(txt))
		sb.WriteString(txt)
		sb.WriteString("\n")
	}

	// 2. 调用跨平台通用 Tesseract OCR 识别整图
	if txt, err := runTesseractOnFile(tmpPath); err == nil && txt != "" {
		sb.WriteString(txt)
		sb.WriteString("\n")
	}

	// 3. 抗留白畸变增强：若原图四周存在大片白边导致文字微缩，纯 Go 自动裁切出文字特写区进行二次复核
	if croppedBytes := autoCropContent(data); len(croppedBytes) > 0 {
		tmpCrop, err := os.CreateTemp("", "secrecy_crop_*.png")
		if err == nil {
			cropPath := tmpCrop.Name()
			tmpCrop.Write(croppedBytes)
			tmpCrop.Close()

			if cropTxt, err := runTesseractOnFile(cropPath); err == nil && cropTxt != "" {
				log.Printf("[OCR] 核心文字区局部特写识别增益: 字符数=%d", len(cropTxt))
				sb.WriteString(cropTxt)
				sb.WriteString("\n")
			}
			os.Remove(cropPath)
		}
	}

	res := normalizeChinese(sb.String())
	return res, nil
}

// autoCropContent 纯 Go 探测图片有效内容边界，消除巨大空白页对 OCR 字符识别的干扰
func autoCropContent(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 100 || h <= 100 {
		return nil
	}

	minX, minY, maxX, maxY := w, h, 0, 0
	nonWhiteFound := false

	step := 1
	if w > 1500 || h > 1500 {
		step = 2
	}

	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bColor, a := img.At(x, y).RGBA()
			// 过滤纯白底色（r,g,b > 60000 且未透明）
			if a > 10000 && (r < 60000 || g < 60000 || bColor < 60000) {
				nonWhiteFound = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if !nonWhiteFound {
		return nil
	}

	padding := 50
	if minX > padding {
		minX -= padding
	} else {
		minX = 0
	}
	if minY > padding {
		minY -= padding
	} else {
		minY = 0
	}
	if maxX+padding < w {
		maxX += padding
	} else {
		maxX = w
	}
	if maxY+padding < h {
		maxY += padding
	} else {
		maxY = h
	}

	cropW := maxX - minX
	cropH := maxY - minY

	// 仅当有效区域占整图面积小于 75% 时进行特写裁切
	if cropW*cropH < (w*h*3)/4 && cropW > 50 && cropH > 50 {
		subImg := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
		draw.Draw(subImg, subImg.Bounds(), img, image.Pt(minX, minY), draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, subImg); err == nil {
			return buf.Bytes()
		}
	}

	return nil
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
		if len(results) >= 2 {
			break // 仅审查前 2 张版头/公文核心图片，防止卡死
		}
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
		// 1. DCTDecode: 标准原生 JPEG 图像（扫描仪、手机拍照公文扫描最常见格式，直接复用无额外CPU消耗）
		if bytes.HasPrefix(stream, []byte("\xFF\xD8\xFF")) || strings.Contains(dict, "DCTDecode") {
			results = append(results, stream)
			continue
		}

		// 2. FlateDecode: 纯像素流（过滤超大像素图，避免纯 Go 像素循环打满 CPU）
		if strings.Contains(dict, "FlateDecode") {
			if width > 1800 || height > 1800 {
				continue
			}
			zr, err := zlib.NewReader(bytes.NewReader(stream))
			if err != nil {
				continue
			}
			raw, _ := io.ReadAll(io.LimitReader(zr, 20<<20))
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
