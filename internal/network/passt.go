package network

import (
	"net"
	"os/exec"
)

// HasPasst checks if passt is available on the system
func HasPasst() bool {
	_, err := exec.LookPath("passt")
	return err == nil
}

// FindFreePort finds an available TCP port on localhost
func FindFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
