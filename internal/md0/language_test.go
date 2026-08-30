package md0

import (
	"strings"
	"testing"
)

func TestLanguageDeclarationIsVersionedMetadata(t *testing.T) {
	doc, err := ParseString("declared.md", "\nmd0: 0.1\n# Report\nValue: @input value number = 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.LanguageVersion != "0.1" || !doc.LanguageDeclared {
		t.Fatalf("language=%q declared=%v", doc.LanguageVersion, doc.LanguageDeclared)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fragment, "md0: 0.1") || !strings.Contains(fragment, "Report") {
		t.Fatalf("language declaration rendered as prose: %s", fragment)
	}
	if inspection := Inspect(doc); !strings.Contains(inspection, "Language            0.1 (declared)") {
		t.Fatalf("inspect missing declared language:\n%s", inspection)
	}
}

func TestLanguageVersionDefaultsAndRejectsUnknownVersions(t *testing.T) {
	doc, err := ParseString("implicit.md", "# Existing document\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.LanguageVersion != "0.1" || doc.LanguageDeclared {
		t.Fatalf("implicit language=%q declared=%v", doc.LanguageVersion, doc.LanguageDeclared)
	}

	for _, source := range []string{"md0:\n# Missing\n", "md0: 9.0\n# Future\n", "md0: 0.1 extra\n"} {
		if _, err := ParseString("unsupported.md", source); err == nil || !strings.Contains(err.Error(), "md0 language") {
			t.Fatalf("source=%q err=%v", source, err)
		}
	}
}
