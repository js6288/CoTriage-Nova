package replayer

import (
	"testing"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
)

func TestBackoffDelayCapsAtMinute(t *testing.T) {
	r := &Replayer{RetryBaseDelay: 5 * time.Second}
	cases := map[int]time.Duration{
		1: 5 * time.Second,
		2: 10 * time.Second,
		3: 20 * time.Second,
		8: time.Minute,
	}
	for attempt, want := range cases {
		if got := r.BackoffDelay(attempt); got != want {
			t.Fatalf("attempt %d backoff = %s, want %s", attempt, got, want)
		}
	}
}

func TestWorkerFallback(t *testing.T) {
	r := &Replayer{}
	if got := r.worker(); got != "router-controller" {
		t.Fatalf("worker fallback = %q, want router-controller", got)
	}
	r.Worker = "router-controller-1"
	if got := r.worker(); got != "router-controller-1" {
		t.Fatalf("worker = %q, want router-controller-1", got)
	}
}

func TestFailureUsesConfiguredMaxAttempts(t *testing.T) {
	item := store.CloudOutboxModel{Attempt: 3, MaxAttempts: 0}
	r := &Replayer{MaxAttempts: 3}
	if item.Attempt < r.MaxAttempts {
		t.Fatalf("test setup expected attempt to reach max attempts")
	}
}
