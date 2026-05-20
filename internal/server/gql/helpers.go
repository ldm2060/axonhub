package gql

import "github.com/ldm2060/axonhub/internal/objects"

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func guidSliceToIntSlice(guids []*objects.GUID) []int {
	ids := make([]int, 0, len(guids))
	for _, g := range guids {
		ids = append(ids, g.ID)
	}
	return ids
}
