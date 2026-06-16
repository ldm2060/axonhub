package objects

// QuotaMonitorConditionOperator is a controlled comparison operator for
// quota-monitor field conditions. It is stored as JSON and never executed as code.
type QuotaMonitorConditionOperator string

const (
	QuotaMonitorOperatorLT          QuotaMonitorConditionOperator = "<"
	QuotaMonitorOperatorLTE         QuotaMonitorConditionOperator = "<="
	QuotaMonitorOperatorEQ          QuotaMonitorConditionOperator = "="
	QuotaMonitorOperatorNEQ         QuotaMonitorConditionOperator = "!="
	QuotaMonitorOperatorGTE         QuotaMonitorConditionOperator = ">="
	QuotaMonitorOperatorGT          QuotaMonitorConditionOperator = ">"
	QuotaMonitorOperatorContains    QuotaMonitorConditionOperator = "contains"
	QuotaMonitorOperatorNotContains QuotaMonitorConditionOperator = "not_contains"
)

// QuotaMonitorBindingCondition describes one structured condition such as
// remaining <= 0. Field can reference parsedData keys, lastPollData keys, or
// the virtual field maxUsageRatio.
type QuotaMonitorBindingCondition struct {
	Field    string                        `json:"field"`
	Operator QuotaMonitorConditionOperator `json:"operator"`
	Value    string                        `json:"value"`
}
