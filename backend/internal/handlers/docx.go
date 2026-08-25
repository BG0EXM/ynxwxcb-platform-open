package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// docxParagraph 一个段落（支持普通文本与加粗）
type docxParagraph struct {
	text  string
	bold  bool
	align string // center / left / right
	size  string // pt 如 "16"
}

// docxBuilder 使用标准库手工构造最小合法 .docx（OOXML），无需第三方依赖
type docxBuilder struct {
	paragraphs []docxParagraph
}

func (d *docxBuilder) add(text string) {
	d.paragraphs = append(d.paragraphs, docxParagraph{text: text, align: "left"})
}

func (d *docxBuilder) addBold(text string) {
	d.paragraphs = append(d.paragraphs, docxParagraph{text: text, bold: true, align: "left"})
}

func (d *docxBuilder) addTitle(text string) {
	d.paragraphs = append(d.paragraphs, docxParagraph{text: text, bold: true, align: "center", size: "22"})
}

func (d *docxBuilder) addSubTitle(text string) {
	d.paragraphs = append(d.paragraphs, docxParagraph{text: text, bold: true, align: "center", size: "18"})
}

func (d *docxBuilder) addEmpty() {
	d.paragraphs = append(d.paragraphs, docxParagraph{text: "", align: "left"})
}

// build 生成 .docx 的 []byte
func (d *docxBuilder) build() ([]byte, error) {
	var body strings.Builder
	body.WriteString(`<w:body>`)
	for _, p := range d.paragraphs {
		body.WriteString(`<w:p>`)
		if p.align != "" && p.align != "left" {
			align := "center"
			if p.align == "right" {
				align = "right"
			}
			body.WriteString(fmt.Sprintf(`<w:pPr><w:jc w:val="%s"/></w:pPr>`, align))
		}
		sz := p.size
		if sz == "" {
			sz = "24"
		}
		boldTag := ""
		if p.bold {
			boldTag = `<w:b/>`
		}
		body.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:rFonts w:ascii="宋体" w:hAnsi="宋体" w:eastAsia="宋体"/><w:sz w:val="%s"/><w:szCs w:val="%s"/>%s</w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`,
			sz, sz, boldTag, xmlEscape(p.text)))
		body.WriteString(`</w:p>`)
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)
	body.WriteString(`</w:body>`)

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + body.String()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, content)
		return err
	}
	if err := write("[Content_Types].xml", contentTypes); err != nil {
		return nil, err
	}
	if err := write("_rels/.rels", rels); err != nil {
		return nil, err
	}
	if err := write("word/document.xml", document); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xmlEscape(s string) string {
	var sb strings.Builder
	_ = xml.EscapeText(&sb, []byte(s))
	return sb.String()
}

// writeDocx 将 docx 数据写入 HTTP 响应
func writeDocx(w http.ResponseWriter, fileName string, data []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fileName, url.PathEscape(fileName)))
	if _, err := w.Write(data); err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
	}
}
