package reconciler

import (
	"context"
	"log"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
)

type NodeLivenessWatcher struct {
	store             storage.Storage
	notReadyThreshold time.Duration
	interval          time.Duration
}

func NewNodeLivenessWatcher(store storage.Storage, threshold, interval time.Duration) *NodeLivenessWatcher {
	return &NodeLivenessWatcher{
		store:             store,
		notReadyThreshold: threshold,
		interval:          interval,
	}
}

func (w *NodeLivenessWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkLiveness(ctx)
		}
	}
}

func (w *NodeLivenessWatcher) checkLiveness(ctx context.Context) {
	nodes, err := w.store.ListNodes(ctx)
	if err != nil {
		log.Printf("Liveness watcher error listing nodes: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, node := range nodes {
		if node.Status == "Ready" && now.Sub(node.LastSeen) > w.notReadyThreshold {
			log.Printf("Node %s missed heartbeats (last seen %v ago); marking NotReady", node.Name, now.Sub(node.LastSeen).Truncate(time.Second))
			if err := w.store.MarkNodeNotReady(ctx, node.Name); err != nil {
				log.Printf("Failed to mark node %s NotReady: %v", node.Name, err)
			}
		}
	}
}

