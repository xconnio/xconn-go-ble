package main

import (
	"fmt"
	"log"

	"github.com/xconnio/xconn-go"
	"github.com/xconnio/xconn-go-ble"
	"tinygo.org/x/bluetooth"
)

type bleManager struct {
	ConnChan      chan xconn.Peer
	advertisement *bluetooth.Advertisement
}

func setupBLE() bleManager {
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

	adapter := bluetooth.DefaultAdapter
	if err = adapter.Enable(); err != nil {
		log.Fatalf("Failed to enable Bluetooth: %v", err)
	}

	adv := adapter.DefaultAdvertisement()
	service := bluetooth.Service{
		UUID: serviceUUID,
	}

	messageChan := make(chan []byte, 1)
	var readerChar bluetooth.Characteristic
	// central → peripheral
	readerCharCfg := bluetooth.CharacteristicConfig{
		Handle: &readerChar,
		UUID:   rxUUID,
		Flags: bluetooth.CharacteristicWritePermission |
			bluetooth.CharacteristicWriteWithoutResponsePermission,
		WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
			messageChan <- value
		},
	}

	service.Characteristics = append(service.Characteristics, readerCharCfg)

	var writerChar bluetooth.Characteristic
	// peripheral → central
	writerCharCfg := bluetooth.CharacteristicConfig{
		Handle: &writerChar,
		UUID:   txUUID,
		Flags: bluetooth.CharacteristicReadPermission |
			bluetooth.CharacteristicNotifyPermission,
	}

	service.Characteristics = append(service.Characteristics, writerCharCfg)

	peerChan := make(chan xconn.Peer, 1)

	var hasConnection bool
	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		if connected {
			if hasConnection {
				// Reject: already busy
				log.Printf("Rejecting extra connection from %s", device.Address.String())
				// Depending on backend, you might need to disconnect manually:
				_ = device.Disconnect()
				return
			}

			hasConnection = true

			peer, err := xconnble.NewPeripheralPeer(messageChan, writerCharCfg)
			if err != nil {
				log.Fatalf("Failed to create peripheral connection: %v", err)
			}

			log.Printf("Device connected: %s", device.Address.String())
			peerChan <- peer
		} else {
			hasConnection = false
			log.Printf("Device disconnected: %s", device.Address.String())
		}
	})

	if err = adapter.AddService(&service); err != nil {
		log.Fatalf("AddService failed: %v", err)
	}

	// Define the peripheral device info.
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "XConnBLE",
		ServiceUUIDs: []bluetooth.UUID{serviceUUID},
	})

	if err != nil {
		log.Fatalf("Failed to configure Bluetooth: %v", err)
	}

	if err = adv.Start(); err != nil {
		log.Fatalf("Failed to start Bluetooth: %v", err)
	}

	println("advertising...")
	return bleManager{
		ConnChan:      peerChan,
		advertisement: adv,
	}
}

func main() {
	man := setupBLE()
	for {
		select {
		case conn := <-man.ConnChan:
			fmt.Println("WE HAVE A CLIENT!", conn)
			for {
				data, err := conn.Read()
				if err != nil {
					break
				}

				fmt.Println(string(data))
			}
		}
	}
}
