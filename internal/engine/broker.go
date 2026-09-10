package engine

import (
	"sync"
	"time"
	pb "gobackup/internal/grpc/pb"
)

// LogBroker manages real-time log distribution to connected gRPC clients
type LogBroker struct {
	mu          sync.Mutex
	subscribers map[chan *pb.LogChunk]bool
}

var GlobalLogBroker = &LogBroker{
	subscribers: make(map[chan *pb.LogChunk]bool),
}

func (b *LogBroker) Subscribe() chan *pb.LogChunk {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *pb.LogChunk, 10000)
	b.subscribers[ch] = true
	return ch
}

func (b *LogBroker) Unsubscribe(ch chan *pb.LogChunk) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

func (b *LogBroker) Broadcast(chunk *pb.LogChunk) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		if chunk.IsSummary || chunk.IsStatus {
			select {
			case ch <- chunk:
			case <-time.After(1 * time.Second):
				// Force wait for up to 1 second for critical UI control messages
			}
		} else {
			select {
			case ch <- chunk:
			default:
				// Drop standard streaming logs if buffer is full
			}
		}
	}
}
