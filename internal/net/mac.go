package driver

import (
	"net"

	"github.com/docker/machine/libmachine/log"
)

// A wrapper around net.HardwareAddr that is marshable as plain text
type HardwareAddr struct {
	net.HardwareAddr
}

// MarshalText implements encoding.TextMarshaler using the
// standard string representation of a HardwareAddr.
func (a HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *HardwareAddr) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*a = HardwareAddr{}
		log.Debugf("MAC: %s", a.String())
		return nil
	}

	v, err := net.ParseMAC(string(text))
	if err != nil {
		*a = HardwareAddr{}
		log.Debugf("MAC: %s", a.String())
		return err
	}

	*a = HardwareAddr{HardwareAddr: v}
	log.Debugf("MAC: %s", a.String())
	return nil
}

func (a *HardwareAddr) ToNetHardwareAddr() net.HardwareAddr {
	return a.HardwareAddr
}

func FromNetHardwareAddr(a net.HardwareAddr) HardwareAddr {
	return HardwareAddr{HardwareAddr: a}
}
