package config

import (
	"github.com/evcc-io/evcc/api"
)

type provider struct {
	meters   handler[api.Meter]
	chargers handler[api.Charger]
	vehicles handler[api.Vehicle]
}

var instance = new(provider)

func init() {
	instance.meters.visited = make(map[string]bool)
	instance.chargers.visited = make(map[string]bool)
	instance.vehicles.visited = make(map[string]bool)
}

func TrackVisitors() {
	instance.meters.TrackVisitors()
}

func AddMeter(conf Named, meter api.Meter) error {
	return instance.meters.Add(conf, meter)
}

func MeterByName(name string) (api.Meter, int, error) {
	return instance.meters.ByName(name)
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

func ChargerByName(name string) (api.Charger, int, error) {
	return instance.chargers.ByName(name)
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

func VehicleByName(name string) (api.Vehicle, int, error) {
	return instance.vehicles.ByName(name)
}

func Vehicles() map[string]api.Vehicle {
	return instance.vehicles.Devices()
}

func VehiclesConfig() []Named {
	return instance.vehicles.Config()
}
