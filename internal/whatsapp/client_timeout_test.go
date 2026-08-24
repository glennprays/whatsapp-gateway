package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
)

// testSendTimeoutConfig builds a minimal config for outboundCtx tests.
func testSendTimeoutConfig(seconds int64) *config.Config {
	return &config.Config{SendTimeoutSeconds: seconds}
}

// TestMapWhatsmeowErr_DeadlineExceeded covers the send-timeout branch: an
// outbound op that outlived SEND_TIMEOUT_SECONDS must surface as
// ErrGatewayTimeout (504), not ErrInternalFailure (500) — and the detection
// must survive the %w wrapping the send helpers apply.
func TestMapWhatsmeowErr_DeadlineExceeded(t *testing.T) {
	c := &client{}
	cases := []struct {
		name string
		err  error
	}{
		{"bare deadline", context.DeadlineExceeded},
		{"wrapped deadline", fmt.Errorf("failed to send message: %w", context.DeadlineExceeded)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := c.mapWhatsmeowErr("trace", "628111", tc.err)
			var de errDomain.Error
			if !errors.As(mapped, &de) {
				t.Fatalf("expected errDomain.Error, got %v", mapped)
			}
			if de.ServiceError() != errDomain.ErrGatewayTimeout {
				t.Fatalf("%v: got %v, want ErrGatewayTimeout", tc.err, de.ServiceError())
			}
		})
	}

	// A plain error (e.g. connection reset) must NOT be misread as a timeout.
	mapped := c.mapWhatsmeowErr("trace", "628111", errors.New("connection reset by peer"))
	var de errDomain.Error
	if !errors.As(mapped, &de) || de.ServiceError() != errDomain.ErrInternalFailure {
		t.Fatalf("plain error should stay internal failure, got %v", mapped)
	}
}

// TestOutboundCtx verifies the derived deadline: bounded when configured,
// pass-through (no cancel side effects) when disabled.
func TestOutboundCtx(t *testing.T) {
	t.Run("disabled when zero", func(t *testing.T) {
		c := &client{}
		ctx, cancel := c.outboundCtx(context.Background())
		defer cancel()
		if ctx != context.Background() {
			t.Fatal("zero timeout must return the caller's ctx unchanged")
		}
	})

	t.Run("bounded by configured seconds", func(t *testing.T) {
		c := &client{cfg: testSendTimeoutConfig(1)}
		ctx, cancel := c.outboundCtx(context.Background())
		defer cancel()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected a deadline")
		}
		if d := time.Until(deadline); d <= 0 || d > 2*time.Second {
			t.Fatalf("deadline %v outside expected ~1s window", d)
		}
	})
}
