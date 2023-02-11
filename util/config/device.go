package config

import (
	"gorm.io/gorm"
)

type Device struct {
	ID            uint   `gorm:"primarykey"`
	Class         string `gorm:"column:class"`
	DeviceConfigs []DeviceConfig
}

type DeviceConfig struct {
	DeviceID   uint `gorm:"primarykey"`
	Key, Value string
}

var db *gorm.DB

func Init(instance *gorm.DB) error {
	db = instance
	return db.AutoMigrate(new(Device), new(DeviceConfig))
}
