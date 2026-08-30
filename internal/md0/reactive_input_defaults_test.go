package md0

import "testing"

func TestReactiveDependentInputDefaultRecomputesWhenParentChanges(t *testing.T) {
	doc, err := ParseString("dependent-input.md", `Base: @input base number = 1
Scaled: @input scaled number = base * 2
Result: {{ scaled }}`)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}

	result, stats, err := session.Update(map[string]string{"base": "3", "scaled": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["scaled"].String(); got != "6" {
		t.Fatalf("scaled=%s, want 6", got)
	}
	if !containsString(stats.Changed, "base") {
		t.Fatalf("changed=%#v, want base", stats.Changed)
	}
	if containsString(stats.Changed, "scaled") {
		t.Fatalf("rendered dependent default was incorrectly treated as a user change: %#v", stats.Changed)
	}
}

func TestReactiveDependentInputCanStillBeExplicitlyOverridden(t *testing.T) {
	doc, err := ParseString("dependent-input-override.md", `Base: @input base number = 1
Scaled: @input scaled number = base * 2`)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}

	result, _, err := session.Update(map[string]string{"base": "1", "scaled": "9"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["scaled"].String(); got != "9" {
		t.Fatalf("scaled=%s, want explicit override 9", got)
	}

	result, _, err = session.Update(map[string]string{"base": "4", "scaled": "9"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["scaled"].String(); got != "9" {
		t.Fatalf("scaled=%s, want retained explicit override 9", got)
	}
}
