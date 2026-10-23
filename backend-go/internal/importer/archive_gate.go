package importer

import (
	"context"
	"sync"
)

// archiveGate serializes operations on one archive across manual jobs, automatic
// tasks and deletion. Independent server/chat archives can still run in parallel.
type archiveGate struct {
	mu    sync.Mutex
	slots map[string]chan struct{}
}

func (g *archiveGate) acquire(ctx context.Context, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	if g.slots == nil {
		g.slots = map[string]chan struct{}{}
	}
	slot := g.slots[id]
	if slot == nil {
		slot = make(chan struct{}, 1)
		g.slots[id] = slot
	}
	g.mu.Unlock()
	select {
	case slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-slot
			return nil, err
		}
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
