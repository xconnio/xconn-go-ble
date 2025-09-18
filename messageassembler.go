package xconnble

import (
	"bytes"
	"sync"
)

type MessageAssembler struct {
	buffer *bytes.Buffer
	mtu    int

	sync.Mutex
}

func NewMessageAssembler(mtu int) *MessageAssembler {
	return &MessageAssembler{
		mtu:    mtu,
		buffer: bytes.NewBuffer(nil),
	}
}

func (m *MessageAssembler) ChunkMessage(message []byte) chan []byte {
	m.Lock()
	defer m.Unlock()

	chunkSize := m.mtu - 1 // 20 bytes MTU, 1 byte for the FIN so we can really send 19 bytes per chunk
	totalChunks := (len(message) + chunkSize - 1) / chunkSize

	chunks := make(chan []byte)

	go func() {
		for i := 0; i < totalChunks; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if i == totalChunks-1 {
				end = len(message)
			}
			chunk := message[start:end]

			var isFinal byte = 0
			if i == totalChunks-1 {
				isFinal = 1
			}

			chunks <- append([]byte{isFinal}, chunk...)
		}
		close(chunks)
	}()

	return chunks
}

func (m *MessageAssembler) Feed(data []byte) []byte {
	m.Lock()
	defer m.Unlock()

	m.buffer.Write(data[1:])
	isFinal := data[0]
	if isFinal == 1 {
		out := m.buffer.Bytes()
		m.buffer.Reset()
		return out
	}

	return nil
}
