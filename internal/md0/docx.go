package md0

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type docxParagraph struct {
	Style string
	Text  string
}

var (
	docxImageRE = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	docxLinkRE  = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
)

// MarshalDOCX exports the evaluated document as an editable Office Open XML
// document using only the Go standard library. PDF remains the fidelity-first
// export; DOCX intentionally favors editable text and semantic content over
// reproducing browser-only SVG visualization geometry.
func MarshalDOCX(doc *Document, result *EvalResult) ([]byte, error) {
	if doc == nil || result == nil {
		return nil, fmt.Errorf("DOCX export requires an evaluated document")
	}
	paragraphs, err := docxParagraphsForNodes(doc.Nodes, result)
	if err != nil {
		return nil, err
	}
	if len(paragraphs) == 0 {
		paragraphs = append(paragraphs, docxParagraph{Text: "md0 document"})
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	files := []struct {
		name string
		body string
	}{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxPackageRels},
		{"word/_rels/document.xml.rels", docxDocumentRels},
		{"word/styles.xml", docxStyles},
		{"word/document.xml", docxDocumentXML(paragraphs)},
	}
	for _, file := range files {
		w, err := zw.Create(file.name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create DOCX part %s: %w", file.name, err)
		}
		if _, err := w.Write([]byte(file.body)); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write DOCX part %s: %w", file.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize DOCX: %w", err)
	}
	return out.Bytes(), nil
}

func docxParagraphsForNodes(nodes []Node, result *EvalResult) ([]docxParagraph, error) {
	var out []docxParagraph
	var walk func([]Node) error
	walk = func(current []Node) error {
		for _, node := range current {
			switch x := node.(type) {
			case MarkdownNode:
				text, err := interpolateMarkdown(x.Text, result.Env)
				if err != nil {
					return fmt.Errorf("line %d: DOCX interpolation: %w", x.Line, err)
				}
				out = append(out, docxParagraphsForMarkdown(text)...)
			case InputNode:
				value, ok := result.Env[x.Name]
				if !ok {
					return fmt.Errorf("line %d: DOCX input %s has no value", x.Line, x.Name)
				}
				label := strings.TrimSpace(x.Prefix)
				if label == "" {
					label = x.Name + ":"
				}
				if !strings.HasSuffix(label, ":") {
					label += ":"
				}
				out = append(out, docxParagraph{Text: label + " " + value.String()})
			case ShowNode:
				value, err := x.Expr.Eval(result.Env)
				if err != nil {
					return err
				}
				out = append(out, docxParagraph{Style: "Code", Text: value.String()})
			case AssertNode:
				assertion, ok := result.AssertionByLine[x.Line]
				if !ok {
					return fmt.Errorf("line %d: DOCX assertion result missing", x.Line)
				}
				state := "PASS"
				if !assertion.Passed {
					state = "FAIL"
				}
				text := state + " — " + assertion.Source
				if assertion.Message != "" {
					text += ": " + assertion.Message
				}
				out = append(out, docxParagraph{Text: text})
			case WhenNode:
				if result.WhenByLine[x.Line] {
					if err := walk(x.Nodes); err != nil {
						return err
					}
				}
			case ChartNode:
				labels, err := x.Labels.Eval(result.Env)
				if err != nil {
					return err
				}
				values, err := x.Values.Eval(result.Env)
				if err != nil {
					return err
				}
				out = append(out, docxParagraph{Text: "Chart " + x.Name + ": " + labels.String() + " — " + values.String()})
			case TableNode:
				columns, err := x.Columns.Eval(result.Env)
				if err != nil {
					return err
				}
				rows, err := x.Rows.Eval(result.Env)
				if err != nil {
					return err
				}
				out = append(out, docxParagraph{Text: "Table " + x.Name + ": " + columns.String()})
				out = append(out, docxParagraph{Text: rows.String()})
			case CalcNode, DataNode:
				// Calculation and data declarations are represented through the
				// prose, inputs, tables, charts, and conditions that consume them.
			default:
				return fmt.Errorf("line %d: unsupported DOCX node", node.LineNo())
			}
		}
		return nil
	}
	if err := walk(nodes); err != nil {
		return nil, err
	}
	return out, nil
}

func docxParagraphsForMarkdown(markdown string) []docxParagraph {
	lines := strings.Split(markdown, "\n")
	out := make([]docxParagraph, 0, len(lines))
	inFence := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			if inFence {
				kind := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if kind != "" {
					out = append(out, docxParagraph{Style: "Code", Text: strings.ToUpper(kind)})
				}
			}
			continue
		}
		if line == "" {
			continue
		}
		if inFence {
			out = append(out, docxParagraph{Style: "Code", Text: line})
			continue
		}

		style := ""
		if strings.HasPrefix(line, "### ") {
			style, line = "Heading3", strings.TrimSpace(line[4:])
		} else if strings.HasPrefix(line, "## ") {
			style, line = "Heading2", strings.TrimSpace(line[3:])
		} else if strings.HasPrefix(line, "# ") {
			style, line = "Heading1", strings.TrimSpace(line[2:])
		}
		line = docxCleanInlineMarkdown(line)
		if line != "" {
			out = append(out, docxParagraph{Style: style, Text: line})
		}
	}
	return out
}

func docxCleanInlineMarkdown(line string) string {
	line = docxImageRE.ReplaceAllString(line, "$1")
	line = docxLinkRE.ReplaceAllString(line, "$1")
	line = strings.NewReplacer(
		"**", "",
		"__", "",
		"`", "",
		"$$", "",
		"$", "",
	).Replace(line)
	return strings.TrimSpace(line)
}

func docxDocumentXML(paragraphs []docxParagraph) string {
	var body strings.Builder
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p>`)
		if paragraph.Style != "" {
			body.WriteString(`<w:pPr><w:pStyle w:val="`)
			body.WriteString(paragraph.Style)
			body.WriteString(`"/></w:pPr>`)
		}
		body.WriteString(`<w:r><w:t xml:space="preserve">`)
		var escaped bytes.Buffer
		_ = xml.EscapeText(&escaped, []byte(paragraph.Text))
		body.WriteString(escaped.String())
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1080" w:right="1080" w:bottom="1080" w:left="1080" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`
}

func docxFilename(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)
	if name == "" || name == "." {
		name = "document"
	}
	return name + ".docx"
}

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`

const docxPackageRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

const docxDocumentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`

const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="30"/><w:szCs w:val="30"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:rPr><w:rFonts w:ascii="Menlo" w:hAnsi="Menlo"/><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr></w:style></w:styles>`
