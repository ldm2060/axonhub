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
	Name         string        `json:"name"`
	Source       string        `json:"source"`
	ChannelID    *string       `json:"channelId,omitempty"`
	ProviderType *string       `json:"providerType,omitempty"` // required when source=template
	ApiKey       *string       `json:"apiKey,omitempty"`       // required when source=template
	ApiURL       string        `json:"apiUrl"`
	ApiMethod    string        `json:"apiMethod"`
	ApiHeaders   string        `json:"apiHeaders"`
	ApiBody      *string       `json:"apiBody,omitempty"`
	PollInterval int           `json:"pollInterval"`
	Fields       []FieldConfig `json:"fields"`
}

type UpdateUsageMonitorChannelInput struct {
	Name         *string        `json:"name,omitempty"`
	ApiURL       *string        `json:"apiUrl,omitempty"`
	ApiMethod    *string        `json:"apiMethod,omitempty"`
	ApiHeaders   *string        `json:"apiHeaders,omitempty"`
	ApiKey       *string        `json:"apiKey,omitempty"` // allow key rotation for template channels
	ApiBody      *string        `json:"apiBody,omitempty"`
	PollInterval *int           `json:"pollInterval,omitempty"`
	Fields       *[]FieldConfig `json:"fields,omitempty"`
	Status       *string        `json:"status,omitempty"`
}

type TestUsageMonitorChannelInput struct {
	ApiURL     string        `json:"apiUrl"`
	ApiMethod  string        `json:"apiMethod"`
	ApiHeaders string        `json:"apiHeaders"`
	ApiBody    *string       `json:"apiBody,omitempty"`
	Fields     []FieldConfig `json:"fields"`
}
