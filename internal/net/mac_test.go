package driver

import (
	"bytes"
	"net"
	"testing"
)

func TestHardwareAddr_UnmarshalText(t *testing.T) {
	tests := []struct {
		msg     string
		text    string
		wantStr string
		wantErr string
	}{
		{
			msg:     "valid mac",
			text:    "aa:bb:cc:dd:ee:ff",
			wantStr: "aa:bb:cc:dd:ee:ff",
			wantErr: "",
		}, {
			msg:     "empty text",
			text:    "",
			wantStr: "",
			wantErr: "",
		}, {
			msg:     "invalid text",
			text:    "foo-bar-baz",
			wantStr: "",
			wantErr: "address foo-bar-baz: invalid MAC address",
		},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			var a HardwareAddr
			err := a.UnmarshalText([]byte(tt.text))
			if tt.wantErr != "" && err.Error() != tt.wantErr {
				t.Errorf("Wanted err %q got %q", tt.wantErr, err.Error())
			} else if tt.wantErr == "" && err != nil {
				t.Errorf("Unexpected error %q", err)
			}

			if tt.wantStr != a.String() {
				t.Errorf("Wanted address string %q but got %q", tt.wantStr, a.String())
			}
		})
	}
}

func TestHardwareAddr_MarshalText(t *testing.T) {
	input := "aa:bb:cc:dd:ee:ff"

	var a HardwareAddr
	if err := a.UnmarshalText([]byte(input)); err != nil {
		t.Fatalf("Unexpected error while unmarashaling: %q", err)
	}

	output, err := a.MarshalText()
	if err != nil {
		t.Fatalf("Error marshaling: %s", err)
	}

	if err := a.UnmarshalText([]byte(input)); err != nil {
		t.Fatalf("Unexpected error while marshaling: %q", err)
	}
	if input != string(output) {
		t.Errorf("Input/Output mismatch: %q and %q", input, string(output))
	}
}

func TestHardwareAddr_ToNetHardwareAddr(t *testing.T) {
	hwAddr := HardwareAddr{}

	nHwAddr := hwAddr.ToNetHardwareAddr()
	if nHwAddr != nil {
		t.Fatalf("Error converting")
	}
}

func TestHardwareAddr_FromNetHardwareAddr(t *testing.T) {
	nHwAddr := net.HardwareAddr{}

	hwAddr := FromNetHardwareAddr(nHwAddr)
	if bytes.Compare(hwAddr.HardwareAddr, nHwAddr) != 0 {
		t.Fatalf("Error converting")
	}

}

func TestHardwareAddr_FromNetHardwareAddrToNetHardwareAddr(t *testing.T) {
	input := "aa:bb:cc:dd:ee:ff"
	nHwAddr, err := net.ParseMAC(input)
	if err != nil {
		t.Fatal("Failed parsing MAC")
	}

	nHwAddr2, err := net.ParseMAC("")
	if err != nil {
		t.Fatal("Failed parsing MAC")
	}

	tests := []struct {
		msg    string
		hwAddr net.HardwareAddr
	}{
		{
			msg:    "nil",
			hwAddr: net.HardwareAddr{},
		},
		{
			msg:    input,
			hwAddr: nHwAddr,
		},
		{
			msg:    "",
			hwAddr: nHwAddr2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			nHwAddr := tt.hwAddr
			hwAddr := FromNetHardwareAddr(nHwAddr)
			if bytes.Compare(nHwAddr, hwAddr.ToNetHardwareAddr()) != 0 {
				t.Error("Values don't match")
			}
		})
	}
}
