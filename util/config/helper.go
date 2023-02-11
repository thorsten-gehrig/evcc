package config

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// Ordered return list of elements ordered by name
func Ordered[T map[string]V, V any](mapp T) []V {
	keys := maps.Keys(mapp)
	slices.Sort(keys)

	res := make([]V, 0, len(mapp))
	for _, k := range keys {
		res = append(res, mapp[k])
	}

	return res
}
