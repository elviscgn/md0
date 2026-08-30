# md0 Runtime Scale Snapshot

This report dogfoods md0 using measurements produced by md0's own Go benchmark harness.

**Environment:** GitHub Actions `ubuntu-24.04`, Go 1.23.12, `linux/amd64`, AMD EPYC 9V74.  
**Recorded benchmark:** One Linux CI benchmark run; the values below remain editable inputs.  
**Command:** `go test ./internal/md0 -run='^$' -bench='^BenchmarkRuntime$' -benchmem -benchtime=50ms -count=1`

> These are one-run microbenchmark measurements, not a cross-machine performance guarantee. The defaults below are recorded data from that run and remain editable so the report itself stays computational.

## 5,000-node recorded timings

Chain parse: @input chain_parse_ms number = 7.745412
Chain plan: @input chain_plan_ms number = 16.683383
Chain eval: @input chain_eval_ms number = 2.063578
Chain render: @input chain_render_ms number = 0.020062
Chain incremental: @input chain_incremental_ms number = 4.050754

Fan-out parse: @input fanout_parse_ms number = 8.691306
Fan-out plan: @input fanout_plan_ms number = 26.054563
Fan-out eval: @input fanout_eval_ms number = 1.996977
Fan-out render: @input fanout_render_ms number = 4.244081
Fan-out incremental: @input fanout_incremental_ms number = 3.853754

@calc chain_frontend_ms = chain_parse_ms + chain_plan_ms + chain_eval_ms + chain_render_ms
@calc fanout_frontend_ms = fanout_parse_ms + fanout_plan_ms + fanout_eval_ms + fanout_render_ms
@calc chain_update_vs_eval = chain_incremental_ms / chain_eval_ms
@calc fanout_update_vs_eval = fanout_incremental_ms / fanout_eval_ms

Approximate isolated-stage sum for the 5,000-node chain: **{{ round(chain_frontend_ms * 1000) / 1000 }} ms**.  
Approximate isolated-stage sum for the 5,000-node fan-out: **{{ round(fanout_frontend_ms * 1000) / 1000 }} ms**.

> The sums above combine independently benchmarked stages; they are useful for orientation but are not a single end-to-end latency measurement.

@assert chain_plan_ms < 100 && fanout_plan_ms < 100
The recorded 5,000-node planning stage exceeded the report's 100 ms sanity budget.
@end

## Stage latency

@chart chain_5000_stage_ms
type = bar
labels = ["Parse", "Plan", "Eval", "Render", "Incremental"]
values = [chain_parse_ms, chain_plan_ms, chain_eval_ms, chain_render_ms, chain_incremental_ms]
@end

@chart fanout_5000_stage_ms
type = bar
labels = ["Parse", "Plan", "Eval", "Render", "Incremental"]
values = [fanout_parse_ms, fanout_plan_ms, fanout_eval_ms, fanout_render_ms, fanout_incremental_ms]
@end

## Full timing curve

@table timing_curve_ms
columns = ["Workload", "Nodes", "Parse ms", "Plan ms", "Eval ms", "Render ms", "Incremental ms"]
rows = [["Chain", 100, 0.136121, 0.212542, 0.027995, 0.003020, 0.063077], ["Chain", 1000, 1.427457, 2.702875, 0.415527, 0.006434, 0.794179], ["Chain", 5000, chain_parse_ms, chain_plan_ms, chain_eval_ms, chain_render_ms, chain_incremental_ms], ["Fan-out", 100, 0.166778, 0.352062, 0.025515, 0.066480, 0.059262], ["Fan-out", 1000, 1.714596, 4.053225, 0.399530, 0.720778, 0.762432], ["Fan-out", 5000, fanout_parse_ms, fanout_plan_ms, fanout_eval_ms, fanout_render_ms, fanout_incremental_ms]]
@end

## 5,000-node allocations

@table allocations_5000
columns = ["Workload", "Stage", "Bytes/op", "Allocs/op"]
rows = [["Chain", "Parse", 2011030, 40031], ["Chain", "Plan", 7629106, 35740], ["Chain", "Eval", 1540960, 147], ["Chain", "Render", 1832, 27], ["Chain", "Incremental", 1966741, 5079], ["Fan-out", "Parse", 2912930, 40049], ["Fan-out", "Plan", 9867037, 56087], ["Fan-out", "Eval", 1540622, 147], ["Fan-out", "Render", 1119001, 24875], ["Fan-out", "Incremental", 2127366, 96]]
@end
