package ratelimiter

import (
	"context"
	"testing"
	"time"
)

// fakeClock is a manually-advanced clock shared by the token bucket and the
// pacer in tests, so a fake sleep that advances it also refills buckets — no
// real sleeping, no spin loops.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestBucket(clk *fakeClock, rate, burst float64) *TokenBucketLimiter {
	tb := NewTokenBucketLimiter(TokenBucketConfig{Rate: rate, Burst: burst, Prefix: "t:"})
	tb.now = clk.Now
	return tb
}

func TestTokenBucket_BurstThenDenyThenRefill(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	tb := newTestBucket(clk, 1, 5) // 5 burst, 1 token/sec
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		res, _ := tb.Allow(ctx, "acc")
		if !res.Allowed {
			t.Fatalf("burst token %d should be allowed", i)
		}
	}
	// 6th is denied with ~1s RetryAfter.
	res, _ := tb.Allow(ctx, "acc")
	if res.Allowed {
		t.Fatal("6th call should be denied (burst exhausted)")
	}
	if res.RetryAfter < 900*time.Millisecond || res.RetryAfter > 1100*time.Millisecond {
		t.Fatalf("RetryAfter for 1 deficit token should be ~1s, got %v", res.RetryAfter)
	}

	// Advance 3s => 3 tokens back.
	clk.Advance(3 * time.Second)
	for i := 0; i < 3; i++ {
		if res, _ := tb.Allow(ctx, "acc"); !res.Allowed {
			t.Fatalf("token %d after 3s refill should be allowed", i)
		}
	}
	if res, _ := tb.Allow(ctx, "acc"); res.Allowed {
		t.Fatal("4th after a 3s refill should be denied")
	}
}

func TestTokenBucket_AllowN(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	tb := newTestBucket(clk, 1, 5)
	ctx := context.Background()

	if res, _ := tb.AllowN(ctx, "acc", 3); !res.Allowed {
		t.Fatal("AllowN(3) with 5 tokens should be allowed")
	}
	// 2 tokens left; AllowN(3) denied with RetryAfter for the 1-token deficit.
	res, _ := tb.AllowN(ctx, "acc", 3)
	if res.Allowed {
		t.Fatal("AllowN(3) with 2 tokens should be denied")
	}
	if res.RetryAfter < 900*time.Millisecond || res.RetryAfter > 1100*time.Millisecond {
		t.Fatalf("deficit of 1 token should be ~1s, got %v", res.RetryAfter)
	}
}

func TestTokenBucket_Reset(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	tb := newTestBucket(clk, 1, 2)
	ctx := context.Background()
	tb.Allow(ctx, "acc")
	tb.Allow(ctx, "acc") // exhausted
	if res, _ := tb.Allow(ctx, "acc"); res.Allowed {
		t.Fatal("should be exhausted before reset")
	}
	if err := tb.Reset(ctx, "acc"); err != nil {
		t.Fatal(err)
	}
	if res, _ := tb.Allow(ctx, "acc"); !res.Allowed {
		t.Fatal("bucket should be full again after Reset")
	}
}
