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
	instance.meters.TrackVisitors()
}

func Meter(name string) (api.Meter, error) {
	return instance.meters.ByName(name)
}

func Meters() map[string]api.Meter {
	return instance.meters.Devices()
}

func MetersConfig() []Named {
	return instance.meters.Config()
}

func Charger(name string) (api.Charger, error) {
	return instance.chargers.ByName(name)
}

func Chargers() map[string]api.Charger {
	return instance.chargers.Devices()
}

func ChargersConfig() []Named {
	return instance.chargers.Config()
}

func Vehicle(name string) (api.Vehicle, error) {
	return instance.vehicles.ByName(name)
}

func Vehicles() map[string]api.Vehicle {
	return instance.vehicles.Devices()
}

func VehiclesConfig() []Named {
	return instance.vehicles.Config()
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
	meters   handler[api.Meter]
	chargers handler[api.Charger]
	vehicles handler[api.Vehicle]
}

func (cp *provider) ConfigureMeters(conf []Named) error {
	cp.meters = handler[api.Meter]{visited: make(map[string]bool)}
	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create meter %d: missing name", i+1)
		}

		m, err := meter.NewFromConfig(cc.Type, cc.Other)
		if err != nil {
			err = fmt.Errorf("cannot create meter '%s': %w", cc.Name, err)
			return err
		}

		if _, err := cp.meters.ByName(cc.Name); err == nil {
			return fmt.Errorf("duplicate meter name: %s already defined and must be unique", cc.Name)
		}

		cp.meters.Add(cc, m)
	}

	return nil
}

func (cp *provider) ConfigureChargers(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.chargers = handler[api.Charger]{visited: make(map[string]bool)}
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

			if _, err := cp.chargers.ByName(cc.Name); err == nil {
				return fmt.Errorf("duplicate charger name: %s already defined and must be unique", cc.Name)
			}

			cp.chargers.Add(cc, c)
			return nil
		})
	}

	return g.Wait()
}

func (cp *provider) ConfigureVehicles(conf []Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	cp.vehicles = handler[api.Vehicle]{visited: make(map[string]bool)}
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

			if _, err := cp.vehicles.ByName(cc.Name); err == nil {
				return fmt.Errorf("duplicate vehicle name: %s already defined and must be unique", cc.Name)
			}

			cp.vehicles.Add(cc, v)
			return nil
		})
	}

	return g.Wait()
}
