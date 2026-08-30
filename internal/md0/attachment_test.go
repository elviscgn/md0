package md0

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONDataAttachmentFeedsCalculations(t *testing.T) {
	doc, err := ParseString("json.md", `@data config json
@calc budget = get(config, "budget")
@calc enabled = get(config, "enabled")
Budget: {{ budget }}
@assert enabled && budget == 1250
configuration loaded
@end`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"budget":1250,"enabled":true,"labels":["a","b"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := BindDataFiles(doc, []string{"config=" + path}); err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Env["budget"].Num != 1250 || len(result.Assertions) != 1 || !result.Assertions[0].Passed {
		t.Fatalf("result=%#v", result)
	}
	graph, err := BuildDependencyGraph(doc)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Producers["config"] != "data:config" {
		t.Fatalf("data producer=%q", graph.Producers["config"])
	}
	snapshot, err := BuildSnapshot(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Data) != 1 || snapshot.Data[0].Name != "config" || snapshot.Data[0].File != "config.json" || len(snapshot.Data[0].SHA256) != 64 {
		t.Fatalf("snapshot data provenance=%#v", snapshot.Data)
	}
}

func TestCSVDataAttachmentSupportsTablesAndColumns(t *testing.T) {
	doc, err := ParseString("csv.md", `@data benchmarks csv
@calc mean_latency = avg(column(benchmarks, "latency_ms"))
Mean: {{ mean_latency }}
@table benchmark_results
columns = columns(benchmarks)
rows = rows(benchmarks)
@end`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "benchmarks.csv")
	if err := os.WriteFile(path, []byte("name,latency_ms,passing\nparser,1.2,true\nrenderer,1.8,false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := BindDataFiles(doc, []string{"benchmarks=" + path}); err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Env["mean_latency"].Num != 1.5 {
		t.Fatalf("mean_latency=%s", result.Env["mean_latency"].String())
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"latency_ms", "parser", "renderer", "1.5"} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("render missing %q: %s", want, fragment)
		}
	}
}

func TestDataAttachmentsMustBeExplicitAndDeclared(t *testing.T) {
	doc, err := ParseString("data.md", "@data report json\n@show report\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := BindDataFiles(doc, nil); err == nil || !strings.Contains(err.Error(), "provide --data report=FILE") {
		t.Fatalf("missing attachment err=%v", err)
	}
	if err := BindDataFiles(doc, []string{"other=/tmp/other.json"}); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("undeclared attachment err=%v", err)
	}
	if _, err := Evaluate(doc, nil); err == nil || !strings.Contains(err.Error(), "no json attachment") {
		t.Fatalf("unbound evaluation err=%v", err)
	}
}

func TestDataAttachmentShapeLimits(t *testing.T) {
	header := make([]string, 65)
	for i := range header {
		header[i] = "c" + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	if _, err := decodeDataCSV([]byte(strings.Join(header, ",") + "\n")); err == nil || !strings.Contains(err.Error(), "1 to 64 columns") {
		t.Fatalf("CSV column limit err=%v", err)
	}

	nested := strings.Repeat("[", maxJSONDepth+2) + "0" + strings.Repeat("]", maxJSONDepth+2)
	if _, err := decodeDataJSON([]byte(nested)); err == nil || !strings.Contains(err.Error(), "nesting levels") {
		t.Fatalf("JSON nesting limit err=%v", err)
	}
}
