package secrecy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// runOCROnFile 调用原生 Apple Vision OCR 工具识别图像或 PDF 页面中的文字
func runOCROnFile(filePath string) (string, error) {
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

	var binPath string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			binPath = c
			break
		}
	}

	if binPath == "" {
		if p, err := exec.LookPath("doc_vision_ocr"); err == nil {
			binPath = p
		}
	}

	// 1. 如果存在 doc_vision_ocr 原生工具，优先执行
	if binPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binPath, filePath)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
	}

	// 2. 跨平台适配：在 Linux / Windows 服务器上自动寻找并调用通用 tesseract OCR 引擎
	if tesseractPath := findTesseractPath(); tesseractPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 优先尝试中英文组合模型
		cmd := exec.CommandContext(ctx, tesseractPath, filePath, "stdout", "-l", "chi_sim+chi_tra+eng")
		if out, err := cmd.Output(); err == nil && len(out) > 0 {
			return string(out), nil
		}
		// 回退尝试 chi_sim
		cmdSim := exec.CommandContext(ctx, tesseractPath, filePath, "stdout", "-l", "chi_sim")
		if out, err := cmdSim.Output(); err == nil && len(out) > 0 {
			return string(out), nil
		}
		// 回退尝试默认模型
		cmdDef := exec.CommandContext(ctx, tesseractPath, filePath, "stdout")
		if out, err := cmdDef.Output(); err == nil && len(out) > 0 {
			return string(out), nil
		}
	}

	return "", nil
}

// findTesseractPath 探测系统中的 tesseract 可执行文件路径（覆盖 Linux, macOS, Windows 常见安装路径）
func findTesseractPath() string {
	if p, err := exec.LookPath("tesseract"); err == nil {
		return p
	}
	candidates := []string{
		"/usr/bin/tesseract",
		"/usr/local/bin/tesseract",
		"/opt/homebrew/bin/tesseract",
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

// runOCROnBytes 将内存中的图像或 PDF 临时写入磁盘并调用 OCR 分析
func runOCROnBytes(ext string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	tmpFile, err := os.CreateTemp("", "secrecy_ocr_*"+ext)
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

	return runOCROnFile(tmpPath)
}
