package config

import (
	"fmt"
)

type handler[T any] struct {
	container []container[T]
	visited   map[string]bool
}

// TrackVisitors tracks visited devices
func (cp *handler[T]) TrackVisitors() {
	cp.visited = make(map[string]bool)
}

// Add adds device and config
func (cp *handler[T]) Add(config Named, device T) {
	cp.container = append(cp.container, container[T]{device: device, config: config})
}

// ByName provides device by name
func (cp *handler[T]) ByName(name string) (T, error) {
	var empty T

	for _, container := range cp.container {
		if name == container.config.Name {
			// track duplicate usage https://github.com/evcc-io/evcc/issues/1744
			if cp.visited != nil {
				if _, ok := cp.visited[name]; ok {
					return empty, fmt.Errorf("duplicate usage: %s", name)
				}
				cp.visited[name] = true
			}

			return container.device, nil
		}
	}

	return empty, fmt.Errorf("does not exist: %s", name)
}

// Devices returns the map of devices
func (cp *handler[T]) Devices() map[string]T {
	res := make(map[string]T, len(cp.container))

	for _, container := range cp.container {
		res[container.config.Name] = container.device
	}

	return res
}

// Config returns the configuration
func (cp *handler[T]) Config() []Named {
	res := make([]Named, 0, len(cp.container))

	for _, container := range cp.container {
		res = append(res, container.config)
	}

	return res
}
