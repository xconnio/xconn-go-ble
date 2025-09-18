package main

import (
	"fmt"
	"log"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/xconnio/xconn-go"
	"github.com/xconnio/xconn-go-ble"
)

func ConnectBLE() (xconn.Peer, error) {
	serviceUUID, err := bluetooth.ParseUUID(xconnble.ServiceUUID)
	if err != nil {
		log.Fatalf("Failed to parse service UUID: %v", err)
	}

	rxUUID, err := bluetooth.ParseUUID(xconnble.RXUUID)
	if err != nil {
		log.Fatalf("Failed to parse RX UUID: %v", err)
	}

	txUUID, err := bluetooth.ParseUUID(xconnble.TXUUID)
	if err != nil {
		log.Fatalf("Failed to parse TX UUID: %v", err)
	}

	var adapter = bluetooth.DefaultAdapter
	if err = adapter.Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable adapter: %w", err)
	}

	deviceChan := make(chan bluetooth.ScanResult, 1)
	err = adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if result.HasServiceUUID(serviceUUID) {
			log.Printf("Found device: %s\n", result.Address.String())
			// Stop scanning
			_ = adapter.StopScan()
			deviceChan <- result
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start BLE scan: %w", err)
	}

	var result bluetooth.ScanResult
	select {
	case scanResult := <-deviceChan:
		result = scanResult
		log.Printf("Found device: %v", result.Address.String())
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for BLE device")
	}

	// Connect
	log.Println("Connecting...")
	device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect BLE device: %w", err)
	}
	defer func() { _ = device.Disconnect() }()
	log.Println("Connected!")

	// Discover service and characteristics
	services, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil || len(services) == 0 {
		return nil, fmt.Errorf("failed to discover BLE services: %w", err)
	}

	service := services[0]
	rxChar, err := service.DiscoverCharacteristics([]bluetooth.UUID{rxUUID})
	if err != nil {
		return nil, fmt.Errorf("rx characteristic not found: %w", err)
	}

	txChar, err := service.DiscoverCharacteristics([]bluetooth.UUID{txUUID})
	if err != nil {
		return nil, fmt.Errorf("tx characteristic not found: %w", err)
	}

	peer, err := xconnble.NewCentralPeer(txChar[0], rxChar[0])
	if err != nil {
		return nil, fmt.Errorf("failed to create BLE peer: %w", err)
	}

	return peer, nil
}

func main() {
	peer, err := ConnectBLE()
	if err != nil {
		log.Fatalf("failed to connect BLE device: %v", err)
	}

	// Example: send some messages to RX
	for i := 1; i <= 5; i++ {
		msg := []byte("Hello peripheral " + time.Now().Format("15:04:05"))
		err = peer.Write(msg)

		if err != nil {
			log.Printf("Write failed: %v", err)
		} else {
			log.Printf("Sent to peripheral: %s", msg)
		}
		time.Sleep(time.Second)
	}

	log.Println("Done. Listening for notifications...")
	select {} // keep running to receive notifications
}
