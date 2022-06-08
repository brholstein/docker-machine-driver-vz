package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	vzog "github.com/Code-Hex/vz"
	"github.com/brholstein/docker-machine-driver-vz/internal/vz"
	"github.com/pkg/errors"
)

func main() {
	var pidFileName string

	flag.CommandLine.Init("", flag.ExitOnError)

	flag.StringVar(&pidFileName, "pid", "", "(Optional) PID file location")

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		log.Fatal("Configuration not provided")
	}

	var config vz.VirtualMachineConfig

	configJson := flag.Arg(0)
	configBytes := []byte(configJson)

	if !json.Valid(configBytes) {
		log.Fatalf("Invalid JSON: '%s'", configJson)
	}

	if err := json.Unmarshal(configBytes, &config); err != nil {
		log.Fatal(errors.Wrap(err, "Failed to parse config"))
	}

	log.Print(config)

	if pidFileName != "" {
		var (
			pidFileHandle *os.File
			err           error
		)

		pidFileHandle, err = os.OpenFile(pidFileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			log.Fatal(errors.Wrap(err, "Unable to create PID file"))
		}
		defer pidFileHandle.Close()
		defer os.Remove(pidFileHandle.Name())

		_, err = pidFileHandle.WriteString(fmt.Sprintf("%d", os.Getpid()))
		if err != nil {
			log.Fatal(errors.Wrap(err, "Failed to write PID file"))
		}
		pidFileHandle.Sync()
	}

	vm, err := vz.NewVirtualMachine(&config)
	if err != nil {
		log.Fatal(err)
	}

	getStateName := func(state vzog.VirtualMachineState) string {
		stateName := "Unknown"
		switch state {
		case vzog.VirtualMachineStateError:
			stateName = "Error"
		case vzog.VirtualMachineStatePaused:
			stateName = "Paused"
		case vzog.VirtualMachineStatePausing:
			stateName = "Pausing"
		case vzog.VirtualMachineStateResuming:
			stateName = "Resuming"
		case vzog.VirtualMachineStateRunning:
			stateName = "Running"
		case vzog.VirtualMachineStateStarting:
			stateName = "Starting"
		case vzog.VirtualMachineStateStopped:
			stateName = "Stopped"
		}
		return stateName
	}

	printCurrentState := func() {
		state := vm.State()
		log.Printf("VM state: %v", getStateName(state))
	}

	go func() {
		printCurrentState()
		for {
			<-vm.StateChangedNotify()
			printCurrentState()
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Kill, os.Interrupt)

	var startError error
	startHandler := func(err error) {
		startError = err
	}

	vm.Start(startHandler)
	if startError != nil {
		log.Fatal(errors.Wrap(startError, "Failed to start VM"))
	}

	sig := <-signals
	log.Println("Received signal:", sig)

	if stopped, err := vm.RequestStop(); !stopped || err != nil {
		log.Fatal(errors.Wrap(err, "Failed to gracefully stop virtual machine"))
	}
}
