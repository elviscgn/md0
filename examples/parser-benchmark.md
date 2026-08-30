# Parser Benchmark

This is a live computational report. The default timings below are illustrative values, not measured md0 benchmark results; replace them with your own benchmark output.

Baseline: @input baseline_ms number = 1.82
Candidate: @input candidate_ms number = 1.31
Allowed regression: @input regression_budget percent = 5

@calc change = (baseline_ms - candidate_ms) / baseline_ms * 100
@calc delta_ms = candidate_ms - baseline_ms
@calc within_budget = candidate_ms <= baseline_ms * (1 + regression_budget / 100)

## Result

Parse time **{{ change >= 0 ? "improved" : "regressed" }} by {{ round(abs(change) * 10) / 10 }}%**.

Absolute delta: **{{ round(delta_ms * 100) / 100 }} ms**.

@when candidate_ms < baseline_ms
The candidate is faster than the baseline at the current input values.
@end

@when candidate_ms >= baseline_ms
The candidate is not faster than the baseline. Check the regression budget before accepting the result.
@end

@assert within_budget
Candidate exceeded the configured regression budget.
@end

@chart latency
type = bar
labels = ["Baseline", "Candidate"]
values = [baseline_ms, candidate_ms]
@end

@table comparison
columns = ["Metric", "Value"]
rows = [["Baseline (ms)", baseline_ms], ["Candidate (ms)", candidate_ms], ["Delta (ms)", delta_ms], ["Improvement (%)", change], ["Regression budget (%)", regression_budget]]
@end
