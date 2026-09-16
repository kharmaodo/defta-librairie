//go:build fts5
// +build fts5

package main

import (
	"context"
	"testing"
	"time"
)

func TestWaitForCoverCleanupStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	if waitForCoverCleanup(ctx, time.Minute) {
		t.Fatal("cancelled cleanup wait returned true")
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancelled cleanup wait did not stop promptly")
	}
}
