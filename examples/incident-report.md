md0: 0.1

# Reliability Review: Checkout Incident

The service measurements are supplied explicitly by the operator; this document cannot discover or open files on its own.

@data services csv

Availability target: @input availability_target percent = 99.85
Latency target: @input latency_target_ms number = 250

@calc mean_availability = avg(column(services, "availability_percent"))
@calc mean_latency = avg(column(services, "latency_ms"))
@calc availability_ok = mean_availability >= availability_target
@calc latency_ok = mean_latency <= latency_target_ms

## Summary

Mean availability was **{{ round(mean_availability * 100) / 100 }}%** and mean latency was **{{ round(mean_latency * 10) / 10 }} ms**.

@when !availability_ok
The incident breached the selected availability target. The follow-up needs an explicit reliability owner and deadline.
@end

@when !latency_ok
Mean latency exceeded the selected target. Include capacity and downstream dependency checks in the remediation plan.
@end

@assert availability_ok && latency_ok
The measured services do not jointly satisfy the selected reliability targets.
@end

@chart service_latency
labels = column(services, "service")
values = column(services, "latency_ms")
@end

@table service_measurements
columns = columns(services)
rows = rows(services)
@end
