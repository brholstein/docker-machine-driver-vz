package vz

import (
	"log"
	"os"

	vznet "github.com/brholstein/docker-machine-driver-vz/internal/net"

	"github.com/Code-Hex/vz"
)

type VirtualMachineConfig struct {
	Kernel            string
	Initrd            string
	CmdLine           string
	CPUs              uint
	Memory            uint
	Disks             []VirtualMachineDiskConfig
	NetworkInterfaces []VirtualMachineNetworkInterface
	SharedDirectories []VirtualMachineSharedDirectory
	// TODO: find a better way to handle this
	SerialPorts []string
}

type VirtualMachineDiskConfig struct {
	Path     string
	ReadOnly bool
}

type VirtualMachineNetworkInterface struct {
	BridgeInterface string
	MACAddress      vznet.HardwareAddr
}

type VirtualMachineSharedDirectory struct {
	Directory string
	Tag       string
}

func (config *VirtualMachineConfig) ConvertToVZ() (*vz.VirtualMachineConfiguration, error) {
	bootLoader := vz.NewLinuxBootLoader(
		config.Kernel,
		vz.WithCommandLine(config.CmdLine),
		vz.WithInitrd(config.Initrd),
	)

	log.Println("BootLoader:", bootLoader)

	vzConfig := vz.NewVirtualMachineConfiguration(
		bootLoader,
		config.CPUs,
		uint64(config.Memory*1024*1024),
	)

	// console
	var serialPorts []*vz.VirtioConsoleDeviceSerialPortConfiguration
	for _, port := range config.SerialPorts {
		var attachment vz.SerialPortAttachment
		var err error
		if port == "-" {
			attachment = vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
		} else {
			attachment, err = vz.NewFileSerialPortAttachment(port, false)
			if err != nil {
				return nil, err
			}
		}
		serialPort := vz.NewVirtioConsoleDeviceSerialPortConfiguration(attachment)
		serialPorts = append(serialPorts, serialPort)
	}
	vzConfig.SetSerialPortsVirtualMachineConfiguration(serialPorts)
	// serialPortAttachment := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	// consoleConfig := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	// config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
	//	consoleConfig,
	// })

	// network
	var networkDevices []*vz.VirtioNetworkDeviceConfiguration
	for _, network := range config.NetworkInterfaces {
		var attachment vz.NetworkDeviceAttachment

		/*
			if network.BridgeInterface {
				net.InterfaceByName()
				bridgedNetwork = vz.BridgedNetwork
				attachment = vz.NewBridgedNetworkDeviceAttachment()

			} else {
		*/
		attachment = vz.NewNATNetworkDeviceAttachment()
		/*
			}
		*/

		networkDevice := vz.NewVirtioNetworkDeviceConfiguration(attachment)

		var macAddr *vz.MACAddress
		if hwAddr := network.MACAddress.ToNetHardwareAddr(); hwAddr != nil {
			macAddr = vz.NewMACAddress(hwAddr)
		} else {
			macAddr = vz.NewRandomLocallyAdministeredMACAddress()
		}
		networkDevice.SetMACAddress(macAddr)
		networkDevices = append(networkDevices, networkDevice)
	}
	vzConfig.SetNetworkDevicesVirtualMachineConfiguration(networkDevices)

	// entropy
	entropyConfig := vz.NewVirtioEntropyDeviceConfiguration()
	vzConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{
		entropyConfig,
	})

	var storageDevices []vz.StorageDeviceConfiguration
	for _, disk := range config.Disks {
		diskImageAttachment, err := vz.NewDiskImageStorageDeviceAttachment(
			disk.Path,
			disk.ReadOnly,
		)

		if err != nil {
			return nil, err
		}
		storageDeviceConfig := vz.NewVirtioBlockDeviceConfiguration(diskImageAttachment)

		storageDevices = append(storageDevices, storageDeviceConfig)
	}
	vzConfig.SetStorageDevicesVirtualMachineConfiguration(storageDevices)

	// traditional memory balloon device which allows for managing guest memory. (optional)
	// vzConfig.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
	// 	vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration(),
	// })

	// socket device (optional)
	// vzConfig.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
	// 	vz.NewVirtioSocketDeviceConfiguration(),
	// })

	var sharedDirectorieConfigs []vz.DirectorySharingDeviceConfiguration
	for _, share := range config.SharedDirectories {
		sharedDirectory := vz.NewSharedDirectory(share.Directory, false)
		singleSharedDirectory := vz.NewSingleDirectoryShare(sharedDirectory)

		sharedDirectoryConfig := vz.NewVirtioFileSystemDeviceConfiguration(share.Tag)
		sharedDirectoryConfig.SetDirectoryShare(singleSharedDirectory)

		sharedDirectorieConfigs = append(sharedDirectorieConfigs, sharedDirectoryConfig)
	}
	vzConfig.SetDirectorySharingDevicesVirtualMachineConfiguration(sharedDirectorieConfigs)

	return vzConfig, nil
}
