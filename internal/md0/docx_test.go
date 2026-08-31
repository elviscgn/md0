package md0

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestMarshalDOCXContainsEvaluatedDocumentText(t *testing.T) {
	doc, err := ParseString("report.md", "# Report\n\nAmplitude: @input amplitude number = 2\n\nResult: **{{ amplitude }}**")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"amplitude": "7"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalDOCX(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 500 {
		t.Fatalf("DOCX unexpectedly small: %d bytes", len(data))
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open DOCX zip: %v", err)
	}
	parts := map[string]string{}
	for _, file := range zr.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[file.Name] = string(body)
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/styles.xml"} {
		if _, ok := parts[name]; !ok {
			t.Fatalf("DOCX missing %s", name)
		}
	}
	for _, want := range []string{"Report", "Amplitude: 7", "Result: 7"} {
		if !strings.Contains(parts["word/document.xml"], want) {
			t.Fatalf("document.xml missing %q: %s", want, parts["word/document.xml"])
		}
	}
}

func TestDOCXFilename(t *testing.T) {
	if got := docxFilename("examples/math-playground.md"); got != "math-playground.docx" {
		t.Fatalf("docxFilename=%q", got)
	}
}
