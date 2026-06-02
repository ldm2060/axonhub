package gql

import "encoding/json"

func structSliceToMapSlice[T any](in []T) []map[string]any {
	out := make([]map[string]any, len(in))
	for i, v := range in {
		raw, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		out[i] = m
	}
	return out
}

func mapSliceToStructSlice[T any](in []map[string]any) []*T {
	out := make([]*T, len(in))
	for i, m := range in {
		raw, err := json.Marshal(m)
		if err != nil {
			continue
		}
		var v T
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		out[i] = &v
	}
	return out
}
