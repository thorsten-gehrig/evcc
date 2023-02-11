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

func AddMeter(conf Named, meter api.Meter) error {
	return instance.meters.Add(conf, meter)
}

func Meter(name string) (api.Meter, error) {
	m, _, err := instance.meters.ByName(name)
	return m, err
}

func MeterID(name string) (int, error) {
	_, id, err := instance.meters.ByName(name)
	return id, err
}

func Meters() map[string]api.Meter {
	return instance.meters.Devices()
}

func MetersConfig() []Named {
	return instance.meters.Config()
}

func AddCharger(conf Named, charger api.Charger) error {
	return instance.chargers.Add(conf, charger)
}

func Charger(name string) (api.Charger, error) {
	c, _, err := instance.chargers.ByName(name)
	return c, err
}

func ChargerID(name string) (int, error) {
	_, id, err := instance.chargers.ByName(name)
	return id, err
}

func Chargers() map[string]api.Charger {
	return instance.chargers.Devices()
}

func ChargersConfig() []Named {
	return instance.chargers.Config()
}

func AddVehicle(conf Named, vehicle api.Vehicle) error {
	return instance.vehicles.Add(conf, vehicle)
}

func Vehicle(name string) (api.Vehicle, error) {
	v, _, err := instance.vehicles.ByName(name)
	return v, err
}

func VehicleID(name string) (int, error) {
	_, id, err := instance.vehicles.ByName(name)
	return id, err
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

		if err := cp.meters.Add(cc, m); err != nil {
			return err
		}
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

			return cp.chargers.Add(cc, c)
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

			return cp.vehicles.Add(cc, v)
		})
	}

	return g.Wait()
}
