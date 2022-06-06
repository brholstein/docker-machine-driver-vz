package vz

import (
	"net"

	"github.com/Code-Hex/vz"
)

func NewVirtualMachine(config *VirtualMachineConfig) (*vz.VirtualMachine, error) {
	vzConfig, err := config.ConvertToVZ()
	if err != nil {
		return nil, err
	}

	validated, err := vzConfig.Validate()
	if !validated || err != nil {
		return nil, err
	}

	vm := vz.NewVirtualMachine(vzConfig)

	return vm, nil
}

func NewRandomLocallyAdministeredHardwareAddr() net.HardwareAddr {
	return vz.NewRandomLocallyAdministeredMACAddress().HardwareAddr()
}
