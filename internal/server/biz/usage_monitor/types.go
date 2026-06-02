package usage_monitor

import "time"

type FieldType string

const (
	FieldTypeJSONPath FieldType = "jsonpath"
	FieldTypeRegex    FieldType = "regex"
)

type FieldFormat string

const (
	FieldFormatPercentage FieldFormat = "percentage"
	FieldFormatFraction   FieldFormat = "fraction"
	FieldFormatNumber     FieldFormat = "number"
	FieldFormatDatetime   FieldFormat = "datetime"
	FieldFormatText       FieldFormat = "text"
)

// FieldConfig is deprecated. Use Variable + DisplayField instead.
type FieldConfig struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	Format       string `json:"format"`
	TotalPath    string `json:"totalPath,omitempty"`
	Unit         string `json:"unit,omitempty"`
	GroupIndex   []int  `json:"groupIndex,omitempty"`
	DisplayOrder int    `json:"displayOrder"`
	Expression   string `json:"expression,omitempty"`
}

// Variable defines how to extract a value from an API response.
type Variable struct {
	Key        string `json:"key"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "jsonpath" | "regex"
	GroupIndex []int  `json:"groupIndex,omitempty"`
}

// DisplayField defines how a value is shown on a monitor card.
type DisplayField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	ValueRef     string `json:"valueRef"` // variable key or expression like "${used}/${total}*100"
	Format       string `json:"format"`   // "percentage" | "fraction" | "number" | "datetime" | "text"
	Unit         string `json:"unit,omitempty"`
	TotalRef     string `json:"totalRef,omitempty"` // variable key for denominator
	DisplayOrder int    `json:"displayOrder"`
	Badge        string `json:"badge,omitempty"`        // badge style key, e.g. "level", "plan"
	BadgePresets string `json:"badgePresets,omitempty"` // JSON map of value->gradient, e.g. '{"lite":"sapphire","pro":"rosegold"}'
}

// VariablesFromFieldConfigs converts deprecated FieldConfigs to Variables.
func VariablesFromFieldConfigs(fcs []FieldConfig) []Variable {
	vars := make([]Variable, 0, len(fcs))
	for _, fc := range fcs {
		if fc.Path == "" {
			continue
		}
		vars = append(vars, Variable{
			Key:        fc.Key,
			Path:       fc.Path,
			Type:       fc.Type,
			GroupIndex: fc.GroupIndex,
		})
	}
	return vars
}

// DisplayFieldsFromFieldConfigs converts deprecated FieldConfigs to DisplayFields.
func DisplayFieldsFromFieldConfigs(fcs []FieldConfig) []DisplayField {
	dfs := make([]DisplayField, 0, len(fcs))
	for _, fc := range fcs {
		valueRef := fc.Key
		if fc.Expression != "" {
			valueRef = fc.Expression
		}
		dfs = append(dfs, DisplayField{
			Key:          fc.Key,
			Label:        fc.Label,
			ValueRef:     valueRef,
			Format:       fc.Format,
			Unit:         fc.Unit,
			TotalRef:     fc.TotalPath,
			DisplayOrder: fc.DisplayOrder,
		})
	}
	return dfs
}

type ParsedField struct {
	Key     string      `json:"key"`
	Label   string      `json:"label"`
	Value   interface{} `json:"value"`
	Total   interface{} `json:"total,omitempty"`
	Percent float64     `json:"percent,omitempty"`
	Unit    string      `json:"unit,omitempty"`
	Format  string      `json:"format"`
	Error   string      `json:"error,omitempty"`
}

type PollData struct {
	Raw      string        `json:"raw"`
	Fields   []ParsedField `json:"fields"`
	PolledAt time.Time     `json:"polledAt"`
}

type TestResult struct {
	Success      bool          `json:"success"`
	RawResponse  string        `json:"rawResponse,omitempty"`
	ParsedFields []ParsedField `json:"parsedFields,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type CreateUsageMonitorChannelInput struct {
	Name          string         `json:"name"`
	Source        string         `json:"source"`
	ChannelID     *string        `json:"channelId,omitempty"`
	ProviderType  *string        `json:"providerType,omitempty"` // required when source=template
	ApiKey        *string        `json:"apiKey,omitempty"`       // required when source=template
	ApiURL        string         `json:"apiUrl"`
	ApiMethod     string         `json:"apiMethod"`
	ApiHeaders    string         `json:"apiHeaders"`
	ApiBody       *string        `json:"apiBody,omitempty"`
	PollInterval  int            `json:"pollInterval"`
	Fields        []FieldConfig  `json:"fields"`
	Variables     []Variable     `json:"variables,omitempty"`
	DisplayFields []DisplayField `json:"displayFields,omitempty"`
}

type UpdateUsageMonitorChannelInput struct {
	Name          *string         `json:"name,omitempty"`
	ApiURL        *string         `json:"apiUrl,omitempty"`
	ApiMethod     *string         `json:"apiMethod,omitempty"`
	ApiHeaders    *string         `json:"apiHeaders,omitempty"`
	ApiKey        *string         `json:"apiKey,omitempty"` // allow key rotation for template channels
	ApiBody       *string         `json:"apiBody,omitempty"`
	PollInterval  *int            `json:"pollInterval,omitempty"`
	Fields        *[]FieldConfig  `json:"fields,omitempty"`
	Variables     *[]Variable     `json:"variables,omitempty"`
	DisplayFields *[]DisplayField `json:"displayFields,omitempty"`
	Status        *string         `json:"status,omitempty"`
}

type TestUsageMonitorChannelInput struct {
	ApiURL     string        `json:"apiUrl"`
	ApiMethod  string        `json:"apiMethod"`
	ApiHeaders string        `json:"apiHeaders"`
	ApiBody    *string       `json:"apiBody,omitempty"`
	Fields     []FieldConfig `json:"fields"`
}
