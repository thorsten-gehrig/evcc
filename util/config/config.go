package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger"
	"github.com/evcc-io/evcc/meter"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/vehicle"
	"github.com/evcc-io/evcc/vehicle/wrapper"
	"golang.org/x/sync/errgroup"
)

var instance = new(provider)

func TrackVisitors() {
	instance.TrackVisitors()
}
func Meter(name string) (api.Meter, error) {
	return instance.Meter(name)
}
func Meters() map[string]api.Meter {
	return instance.Meters()
}
func MetersConfig() []Named {
	return instance.MetersConfig()
}

func Charger(name string) (api.Charger, error) {
	return instance.Charger(name)
}
func Chargers() map[string]api.Charger {
	return instance.Chargers()
}
func ChargersConfig() []Named {
	return instance.ChargersConfig()
}

func Vehicle(name string) (api.Vehicle, error) {
	return instance.Vehicle(name)
}
func Vehicles() map[string]api.Vehicle {
	return instance.Vehicles()
}
func VehiclesConfig() []Named {
	return instance.VehiclesConfig()
}

func ConfigureMeters(conf []Named) error {
	return instance.ConfigureMeters(conf)
}
func ConfigureChargers(conf []Named) error {
	return instance.ConfigureChargers(conf)
}
func ConfigureVehicles(conf []Named) error {
	return instance.ConfigureVehicles(conf)
}

type provider struct {
	meters   map[string]container[api.Meter]
	chargers map[string]container[api.Charger]
	vehicles map[string]container[api.Vehicle]
	visited  map[string]bool
}

func (cp *provider) TrackVisitors() {
	cp.visited = make(map[string]bool)
}

// Meter provides meters by name
func (cp *provider) Meter(name string) (api.Meter, error) {
	if meter, ok := cp.meters[name]; ok {
		// track duplicate usage https://github.com/evcc-io/evcc/issues/1744
		if cp.visited != nil {
			if _, ok := cp.visited[name]; ok {
				return nil, fmt.Errorf("duplicate meter usage: %s", name)
			}
			cp.visited[name] = true
		}

		return meter.device, nil
	}
	return nil, fmt.Errorf("meter does not exist: %s", name)
}

// Meters returns the map configured of meters
func (cp *provider) Meters() map[string]api.Meter {
	return deviceMap(cp.meters)
}

// MetersConfig returns the map configured of meters
func (cp *provider) MetersConfig() []Named {
	return configMap(cp.meters)
}

// Charger provides chargers by name
func (cp *provider) Charger(name string) (api.Charger, error) {
	if charger, ok := cp.chargers[name]; ok {
		return charger.device, nil
	}
	return nil, fmt.Errorf("charger does not exist: %s", name)
}

// Chargers returns the map configured of chargers
func (cp *provider) Chargers() map[string]api.Charger {
	return deviceMap(cp.chargers)
}

// ChargersConfig returns the map configured of chargers
func (cp *provider) ChargersConfig() []Named {
	return configMap(cp.chargers)
}

// Vehicle provides vehicles by name
func (cp *provider) Vehicle(name string) (api.Vehicle, error) {
	if vehicle, ok := cp.vehicles[name]; ok {
		return vehicle.device, nil
	}
	return nil, fmt.Errorf("vehicle does not exist: %s", name)
}

// Vehicles returns the map configured of vehicles
func (cp *provider) Vehicles() map[string]api.Vehicle {
	return deviceMap(cp.vehicles)
}

// VehiclesConfig returns the map configured of vehicles
func (cp *provider) VehiclesConfig() []Named {
	return configMap(cp.vehicles)
}

func (cp *provider) ConfigureMeters(conf []Named) error {
	cp.meters = make(map[string]container[api.Meter])
	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create meter %d: missing name", i+1)
		}

		m, err := meter.NewFromConfig(cc.Type, cc.Other)
		if err != nil {
			err = fmt.Errorf("cannot create meter '%s': %w", cc.Name, err)
			return err
		}

		if _, exists := cp.meters[cc.Name]; exists {
			return fmt.Errorf("duplicate meter name: %s already defined and must be unique", cc.Name)
		}

		cp.meters[cc.Name] = container[api.Meter]{cc, m}
	}

	return nil
}

func (cp *provider) ConfigureChargers(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.chargers = make(map[string]container[api.Charger])
	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create charger %d: missing name", i+1)
		}

		cc := cc

		g.Go(func() error {
			c, err := charger.NewFromConfig(cc.Type, cc.Other)
			if err != nil {
				return fmt.Errorf("cannot create charger '%s': %w", cc.Name, err)
			}

			mu.Lock()
			defer mu.Unlock()

			if _, exists := cp.chargers[cc.Name]; exists {
				return fmt.Errorf("duplicate charger name: %s already defined and must be unique", cc.Name)
			}

			cp.chargers[cc.Name] = container[api.Charger]{cc, c}
			return nil
		})
	}

	return g.Wait()
}

func (cp *provider) ConfigureVehicles(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.vehicles = make(map[string]container[api.Vehicle])
	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create vehicle %d: missing name", i+1)
		}

		cc := cc

		g.Go(func() error {
			v, err := vehicle.NewFromConfig(cc.Type, cc.Other)
			if err != nil {
				var ce *util.ConfigError
				if errors.As(err, &ce) {
					return fmt.Errorf("cannot create vehicle '%s': %w", cc.Name, err)
				}

				// wrap non-config vehicle errors to prevent fatals
				v = wrapper.New(err)
			}

			// ensure vehicle config has title
			if v.Title() == "" {
				//lint:ignore SA1019 as Title is safe on ascii
				v.SetTitle(strings.Title(cc.Name))
			}

			mu.Lock()
			defer mu.Unlock()

			if _, exists := cp.vehicles[cc.Name]; exists {
				return fmt.Errorf("duplicate vehicle name: %s already defined and must be unique", cc.Name)
			}

			cp.vehicles[cc.Name] = container[api.Vehicle]{cc, v}
			return nil
		})
	}

	return g.Wait()
}
