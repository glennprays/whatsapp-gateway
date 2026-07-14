package whatsapp_usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// newTestUsecase builds a usecase carrying only the read cache + budget, enough
// to exercise queryWithBudget without the full dependency graph.
func newTestUsecase(cacheTTL time.Duration, budget int64) *WhatsappMessageUsecase {
	return &WhatsappMessageUsecase{
		queryCache: newTTLCache(cacheTTL),
		queryBudget: ratelimiter.NewMemoryLimiter(ratelimiter.Config{
			Limit:  budget,
			Window: time.Minute,
			Prefix: "test:readquery:",
		}),
	}
}

func TestQueryWithBudget_CachesAfterMiss(t *testing.T) {
	uc := newTestUsecase(time.Minute, 100)
	calls := 0
	fn := func() (string, error) { calls++; return "value", nil }

	got, err := queryWithBudget(uc, context.Background(), "628123", "k", fn)
	if err != nil || got != "value" {
		t.Fatalf("first call: got %q err %v", got, err)
	}
	got, err = queryWithBudget(uc, context.Background(), "628123", "k", fn)
	if err != nil || got != "value" {
		t.Fatalf("second call: got %q err %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("expected fn called once (cache hit on 2nd), got %d", calls)
	}
}

func TestQueryWithBudget_ExhaustsBudget(t *testing.T) {
	uc := newTestUsecase(time.Minute, 2)
	calls := 0
	// Distinct cache keys so every call is a miss and spends a token.
	call := func(key string) error {
		_, err := queryWithBudget(uc, context.Background(), "628123", key,
			func() (string, error) { calls++; return "v", nil })
		return err
	}

	if err := call("a"); err != nil {
		t.Fatalf("call a: %v", err)
	}
	if err := call("b"); err != nil {
		t.Fatalf("call b: %v", err)
	}
	err := call("c") // budget of 2 exhausted
	if err == nil {
		t.Fatal("expected budget-exceeded error on 3rd distinct read")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrTooManyRequests {
		t.Fatalf("expected ErrTooManyRequests, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected fn called twice (3rd blocked by budget), got %d", calls)
	}
}

func TestGetGroupInfo_RejectsNonGroupChat(t *testing.T) {
	uc := newTestUsecase(time.Minute, 100)
	// A bare number resolves to @s.whatsapp.net, which is not a group; the guard
	// must reject with 400 before ever touching the (nil) manager.
	_, err := uc.GetGroupInfo(context.Background(), "trace", "628111", "6282222222222")
	if err == nil {
		t.Fatal("expected 400 for non-group chat")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestCommunityReads_RejectNonGroupChat(t *testing.T) {
	// Both community reads must reject a non-@g.us chat with 400 via the shared
	// resolveGroupJID guard, before ever touching the (nil) manager.
	badChats := []string{"6282222222222", "6282222222222@s.whatsapp.net", "1122334455@lid"}
	for _, chat := range badChats {
		t.Run("subgroups/"+chat, func(t *testing.T) {
			uc := newTestUsecase(time.Minute, 100)
			_, err := uc.ListSubGroups(context.Background(), "trace", "628111", chat)
			assertBadRequest(t, err)
		})
		t.Run("participants/"+chat, func(t *testing.T) {
			uc := newTestUsecase(time.Minute, 100)
			_, err := uc.ListCommunityParticipants(context.Background(), "trace", "628111", chat)
			assertBadRequest(t, err)
		})
	}
}

func TestGetContactInfo_RejectsGroupChat(t *testing.T) {
	uc := newTestUsecase(time.Minute, 100)
	_, err := uc.GetContactInfo(context.Background(), "trace", "628111", "120363000000000000@g.us", "")
	if err == nil {
		t.Fatal("expected 400 for group chat")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestQueryWithBudget_DoesNotCacheErrors(t *testing.T) {
	uc := newTestUsecase(time.Minute, 100)
	calls := 0
	fn := func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("transient")
		}
		return "ok", nil
	}

	if _, err := queryWithBudget(uc, context.Background(), "628123", "k", fn); err == nil {
		t.Fatal("expected error on first call")
	}
	got, err := queryWithBudget(uc, context.Background(), "628123", "k", fn)
	if err != nil || got != "ok" {
		t.Fatalf("retry after error: got %q err %v", got, err)
	}
	if calls != 2 {
		t.Fatalf("expected fn re-invoked after error (not cached), got %d calls", calls)
	}
}
