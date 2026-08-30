md0: 0.1

# Budget and Growth Scenario

Base financial assumptions are attached as reviewed JSON. Interactive inputs represent the decisions being explored.

@data assumptions json

Revenue growth: @input growth_percent percent = 12
Cost inflation: @input inflation_percent percent = 6
Additional hiring: @input hiring_cost currency = 18000
Contingency: @input contingency currency = 8000

@calc base_revenue = get(assumptions, "base_revenue")
@calc base_cost = get(assumptions, "base_cost")
@calc projected_revenue = base_revenue * (1 + growth_percent / 100)
@calc projected_cost = base_cost * (1 + inflation_percent / 100) + hiring_cost + contingency
@calc projected_margin = projected_revenue - projected_cost
@calc margin_percent = projected_margin / projected_revenue * 100
@calc minimum_margin = get(assumptions, "minimum_margin_percent")

## Projection

Projected revenue is **{{ round(projected_revenue) }}** against projected cost of **{{ round(projected_cost) }}**. The resulting margin is **{{ round(margin_percent * 10) / 10 }}%**.

@when margin_percent < minimum_margin
This scenario falls below the approved minimum margin. Reduce discretionary cost or revisit the growth assumption.
@end

@assert margin_percent >= minimum_margin
Projected margin is below the minimum approved by the planning team.
@end

@chart scenario
labels = ["Revenue", "Cost", "Margin"]
values = [projected_revenue, projected_cost, projected_margin]
@end

@table scenario_summary
columns = ["Metric", "Value"]
rows = [["Base revenue", base_revenue], ["Base cost", base_cost], ["Projected revenue", projected_revenue], ["Projected cost", projected_cost], ["Projected margin", projected_margin], ["Margin percent", margin_percent]]
@end
