md0: 0.1

# Engineering Decision Record: Queue Strategy

Use this document to compare two implementation options while keeping the assumptions and acceptance checks reviewable in Git.

Option A setup cost: @input option_a_setup currency = 18000
Option A monthly cost: @input option_a_monthly currency = 2400
Option B setup cost: @input option_b_setup currency = 9000
Option B monthly cost: @input option_b_monthly currency = 3900
Evaluation period: @input months integer = 18
Maximum approved spend: @input budget currency = 85000

@calc option_a_total = option_a_setup + option_a_monthly * months
@calc option_b_total = option_b_setup + option_b_monthly * months
@calc recommended = option_a_total <= option_b_total ? "Option A" : "Option B"
@calc recommended_total = option_a_total <= option_b_total ? option_a_total : option_b_total
@calc savings = abs(option_a_total - option_b_total)

## Recommendation

Choose **{{ recommended }}**. Over {{ months }} months, the lower-cost option is projected to cost **{{ recommended_total }}**, saving **{{ savings }}** against the alternative.

@when recommended == "Option A"
Option A has the higher setup cost but becomes cheaper over the selected operating period.
@end

@when recommended == "Option B"
Option B minimizes cost over the selected operating period.
@end

@assert recommended_total <= budget
Neither option fits the approved budget. Revisit scope, pricing, or the evaluation period.
@end

@chart total_cost
labels = ["Option A", "Option B", "Budget"]
values = [option_a_total, option_b_total, budget]
@end

@table decision_summary
columns = ["Option", "Setup", "Monthly", "Period total"]
rows = [["Option A", option_a_setup, option_a_monthly, option_a_total], ["Option B", option_b_setup, option_b_monthly, option_b_total]]
@end
