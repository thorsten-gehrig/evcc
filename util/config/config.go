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

// Provider provides configuration items
type Provider struct {
	meters   map[string]api.Meter
	chargers map[string]api.Charger
	vehicles map[string]api.Vehicle
	visited  map[string]bool
}

func (cp *Provider) TrackVisitors() {
	cp.visited = make(map[string]bool)
}

// Meter provides meters by name
func (cp *Provider) Meter(name string) (api.Meter, error) {
	if meter, ok := cp.meters[name]; ok {
		// track duplicate usage https://github.com/evcc-io/evcc/issues/1744
		if cp.visited != nil {
			if _, ok := cp.visited[name]; ok {
				return nil, fmt.Errorf("duplicate meter usage: %s", name)
			}
			cp.visited[name] = true
		}

		return meter, nil
	}
	return nil, fmt.Errorf("meter does not exist: %s", name)
}

// Meters returns the map configured of meters
func (cp *Provider) Meters() map[string]api.Meter {
	return cp.meters
}

// Charger provides chargers by name
func (cp *Provider) Charger(name string) (api.Charger, error) {
	if charger, ok := cp.chargers[name]; ok {
		return charger, nil
	}
	return nil, fmt.Errorf("charger does not exist: %s", name)
}

// Chargers returns the map configured of chargers
func (cp *Provider) Chargers() map[string]api.Charger {
	return cp.chargers
}

// Vehicle provides vehicles by name
func (cp *Provider) Vehicle(name string) (api.Vehicle, error) {
	if vehicle, ok := cp.vehicles[name]; ok {
		return vehicle, nil
	}
	return nil, fmt.Errorf("vehicle does not exist: %s", name)
}

// Vehicles returns the map configured of vehicles
func (cp *Provider) Vehicles() map[string]api.Vehicle {
	return cp.vehicles
}

func (cp *Provider) ConfigureMeters(conf []Named) error {
	cp.meters = make(map[string]api.Meter)
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

		cp.meters[cc.Name] = m
	}

	return nil
}

func (cp *Provider) ConfigureChargers(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.chargers = make(map[string]api.Charger)
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

			cp.chargers[cc.Name] = c
			return nil
		})
	}

	return g.Wait()
}

func (cp *Provider) ConfigureVehicles(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.vehicles = make(map[string]api.Vehicle)
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

			cp.vehicles[cc.Name] = v
			return nil
		})
	}

	return g.Wait()
}
