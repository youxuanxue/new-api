package common

import (
	"testing"
	"time"
)

func TestGracefulShutdownDrainMatchesFifteenSeconds(t *testing.T) {
	if GracefulShutdownDrain != 15*time.Second {
		t.Fatalf("expected 15s graceful shutdown drain, got %v", GracefulShutdownDrain)
	}
}
