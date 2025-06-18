package main

import (
	"fmt"
	"interface/src/serial"
	"interface/src/server"
)

func main() {
	serialState, err := serial.NewState(serial.Default_serial)
	if err != nil {
		fmt.Printf("Failed to initialize state. Error: %s\n", err)
	}

	go serialState.SerialService()
	go server.StartServer(serialState)
	fmt.Println("Service dispatched")
	for {
	}
}
