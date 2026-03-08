package engine

import "net"

// GetFreePort asks the operating system kernel for a free, available ephemeral port.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	
	// The port is immediately returned to the system when listener closes, 
	// but it's highly likely to remain available for the next ms when libp2p binds.
	return l.Addr().(*net.TCPAddr).Port, nil
}
