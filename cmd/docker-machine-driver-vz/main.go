//go:build darwin
// +build darwin

package main

import (
	"github.com/brholstein/docker-machine-driver-vz/internal/driver"
	"github.com/docker/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(driver.NewDriver("", ""))
}
