package runtime

import (
	"testing"
	"time"

	"github.com/hashicorp/go-plugin"
)

// After a failed bind-time dial the state holds a broker and a stream id but
// no client. host() must return nil without dialing again: the host's
// connection info has expired by then, and a retry would hold s.mu for up to
// go-plugin's five-second wait. The zero-value broker has no stream plumbing,
// so any Dial on it panics or blocks and this test fails.
func TestHostDoesNotRedialAfterFailedBind(t *testing.T) {
	s := &pluginHostState{broker: &plugin.GRPCBroker{}, brokerID: 7}

	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		if got := s.host(); got != nil {
			t.Errorf("host() = %v, want nil after a failed bind-time dial", got)
		}
	}()
	select {
	case r := <-done:
		if r != nil {
			t.Fatalf("host() dialed the broker: %v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("host() blocked; it must not dial the broker")
	}
}
