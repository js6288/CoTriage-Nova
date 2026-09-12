package runtime

import (
	"context"
	"fmt"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
	"github.com/redis/go-redis/v9"
)

type QueueSnapshot struct {
	InferredPending int64 `json:"inferred_pending"`
	CloudReqLength  int64 `json:"cloud_req_length"`
	OutboxPending   int64 `json:"outbox_pending"`
}

type QueueWatcher struct {
	Redis          *redis.Client
	Store          *store.Store
	InferredStream string
	ConsumerGroup  string
	CloudReqStream string
}

func (w QueueWatcher) Snapshot(ctx context.Context) (QueueSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var snap QueueSnapshot
	var errs []error
	if w.Redis != nil {
		if groups, err := w.Redis.XInfoGroups(ctx, w.InferredStream).Result(); err == nil {
			for _, group := range groups {
				if group.Name == w.ConsumerGroup {
					snap.InferredPending = group.Pending
					break
				}
			}
		} else if err != redis.Nil {
			errs = append(errs, fmt.Errorf("xinfo groups inferred: %w", err))
		}
		if n, err := w.Redis.XLen(ctx, w.CloudReqStream).Result(); err == nil {
			snap.CloudReqLength = n
		} else if err != redis.Nil {
			errs = append(errs, fmt.Errorf("xlen cloud req: %w", err))
		}
	}
	if w.Store != nil {
		if n, err := w.Store.CountPendingOutbox(ctx); err == nil {
			snap.OutboxPending = n
		} else {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return snap, errs[0]
	}
	return snap, nil
}
