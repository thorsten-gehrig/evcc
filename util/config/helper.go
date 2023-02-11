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

// deviceMap returns the devices from given map of containers
func deviceMap[T map[string]container[V], V any](mapp T) map[string]V {
	res := make(map[string]V, len(mapp))

	for k, v := range mapp {
		res[k] = v.device
	}

	return res
}

// configMap returns the devices from given map of containers
func configMap[T map[string]container[V], V any](mapp T) []Named {
	res := make([]Named, 0, len(mapp))

	for _, v := range mapp {
		res = append(res, v.config)
	}

	return res
}
