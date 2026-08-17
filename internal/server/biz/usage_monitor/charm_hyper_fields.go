package usage_monitor

import (
	"encoding/json"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// CharmHyperQuotaToParsedFields converts a Charm Hyper QuotaData into ParsedFields
// consumable by the generic usage-monitor storage + display path and DeriveQuotaStatus.
// The checker stores the credit balance under QuotaData.RawData["balance"].
func CharmHyperQuotaToParsedFields(q provider_quota.QuotaData) []ParsedField {
	if q.RawData == nil {
		return nil
	}

	balance, ok := q.RawData["balance"]
	if !ok {
		return nil
	}

	balanceF, _ := toFloat(balance)

	return []ParsedField{
		{
			Key:    "balance",
			Label:  "Credit Balance",
			Value:  balanceF,
			Format: string(FieldFormatNumber),
		},
	}
}

// CharmHyperQuotaRawJSON returns the checker's RawData as a JSON string for the
// PollData.Raw field (used by the degraded last_poll_data display path).
func CharmHyperQuotaRawJSON(q provider_quota.QuotaData) string {
	if q.RawData == nil {
		return "{}"
	}
	b, err := json.Marshal(q.RawData)
	if err != nil {
		return "{}"
	}
	return string(b)
}
