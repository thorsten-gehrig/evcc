package config

import (
	"fmt"

	"gorm.io/gorm"
)

type Device struct {
	ID      uint `gorm:"primarykey"`
	Class   Class
	Type    string
	Details []DeviceDetail
}

// AsMap converts device details to map
func (d *Device) AsMap() map[string]any {
	res := make(map[string]any)
	for _, detail := range d.Details {
		res[detail.Key] = detail.Value
	}
	return res
}

type DeviceDetail struct {
	DeviceID uint   `gorm:"primarykey"`
	Key      string `gorm:"primarykey"`
	Value    string
}

var db *gorm.DB

func Init(instance *gorm.DB) error {
	db = instance
	return db.AutoMigrate(new(Device), new(DeviceDetail))
}

// Devices returns devices by class from the database
func Devices(class Class) ([]Device, error) {
	var devices []Device
	tx := db.Where(&Device{Class: class}).Find(&devices)

	// remove devices without details
	for i := 0; i < len(devices); {
		d := devices[i]
		if len(d.Details) > 0 {
			i++
			continue
		}

		// delete device
		copy(devices[i:], devices[i+1:])
		devices = devices[: len(devices)-1 : len(devices)-1]
	}

	return devices, tx.Error
}

// DeviceByID returns device by id from the database
func DeviceByID(id int) (Device, error) {
	var device Device
	tx := db.Where(&Device{ID: uint(id)}).First(&device)
	return device, tx.Error
}

// AddDevice adds a new device to the database
func AddDevice(class Class, typ string, config map[string]any) (uint, error) {
	device := Device{Class: class, Type: typ}
	if tx := db.Create(&device); tx.Error != nil {
		return 0, tx.Error
	}

	var devices []DeviceDetail
	for k, v := range config {
		devices = append(devices, DeviceDetail{
			DeviceID: device.ID, Key: k, Value: fmt.Sprintf("%v", v),
		})
	}

	tx := db.Create(&devices)
	return device.ID, tx.Error
}
