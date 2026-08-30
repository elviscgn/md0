# Parser Benchmark

Baseline: @input baseline_ms number = 1.82
Candidate: @input candidate_ms number = 1.31

@calc change = (baseline_ms - candidate_ms) / baseline_ms * 100

## Result

Parse time **{{ change >= 0 ? "improved" : "regressed" }} by {{ round(abs(change) * 10) / 10 }}%**.

@assert candidate_ms <= baseline_ms * 1.05
Candidate regressed by more than the allowed 5%.
@end

@chart latency
type = bar
labels = ["Baseline", "Candidate"]
values = [baseline_ms, candidate_ms]
@end

@table comparison
columns = ["Metric", "Baseline", "Candidate"]
rows = [["Parse latency (ms)", baseline_ms, candidate_ms]]
@end
