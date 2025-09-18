package xconnble

import (
	"net"

	"tinygo.org/x/bluetooth"

	"github.com/xconnio/xconn-go"
)

type CentralPeer struct {
	writer bluetooth.DeviceCharacteristic

	messageChan chan []byte
	assembler   *MessageAssembler
}

func NewCentralPeer(reader, writer bluetooth.DeviceCharacteristic) (xconn.Peer, error) {
	messageChan := make(chan []byte, 1)
	assembler := NewMessageAssembler(20)
	err := reader.EnableNotifications(func(buf []byte) {
		toSend := assembler.Feed(buf)
		if toSend != nil {
			messageChan <- toSend
		}
	})

	if err != nil {
		return nil, err
	}

	return &CentralPeer{
		writer:      writer,
		messageChan: messageChan,
		assembler:   assembler,
	}, nil
}

func (c *CentralPeer) Type() xconn.TransportType {
	return xconn.TransportNone
}

func (c *CentralPeer) NetConn() net.Conn {
	return nil
}

func (c *CentralPeer) Read() ([]byte, error) {
	return <-c.messageChan, nil
}

func (c *CentralPeer) Write(bytes []byte) error {
	for chunk := range c.assembler.ChunkMessage(bytes) {
		if _, err := c.writer.WriteWithoutResponse(chunk); err != nil {
			return err
		}
	}

	return nil
}

type PeripheralPeer struct {
	writer bluetooth.CharacteristicConfig

	messageChan chan []byte
	assembler   *MessageAssembler
}

func NewPeripheralPeer(reader, writer bluetooth.CharacteristicConfig) (xconn.Peer, error) {
	messageChan := make(chan []byte, 1)
	assembler := NewMessageAssembler(20)
	reader.WriteEvent = func(client bluetooth.Connection, offset int, value []byte) {
		toSend := assembler.Feed(value)
		if toSend != nil {
			messageChan <- toSend
		}
	}

	return &PeripheralPeer{
		writer:      writer,
		messageChan: messageChan,
		assembler:   assembler,
	}, nil
}

func (p *PeripheralPeer) Type() xconn.TransportType {
	return xconn.TransportNone
}

func (p *PeripheralPeer) NetConn() net.Conn {
	return nil
}

func (p *PeripheralPeer) Read() ([]byte, error) {
	return <-p.messageChan, nil
}

func (p *PeripheralPeer) Write(bytes []byte) error {
	for chunk := range p.assembler.ChunkMessage(bytes) {
		if _, err := p.writer.Handle.Write(chunk); err != nil {
			return err
		}
	}

	return nil
}
