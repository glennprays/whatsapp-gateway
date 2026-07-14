package whatsapp

import (
	"context"
	"testing"
	"time"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// mockSessionRepo captures the last UpsertSessionStatus call. The webhook
// methods are unused by the lifecycle-event paths under test.
type mockSessionRepo struct {
	called bool
	phone  string
	state  string
	reason string
	banExp *time.Time
}

func (m *mockSessionRepo) SetWebhook(ctx context.Context, jid, url, secret string) error { return nil }
func (m *mockSessionRepo) GetWebhook(ctx context.Context, jid string) (*waDomain.Webhook, error) {
	return nil, nil
}
func (m *mockSessionRepo) GetWebhookByPhone(ctx context.Context, phone string) (*waDomain.Webhook, string, error) {
	return nil, "", nil
}
func (m *mockSessionRepo) DeleteWebhook(ctx context.Context, jid string) error { return nil }
func (m *mockSessionRepo) SetWebhookSubscription(ctx context.Context, jid, url, hmacSecret, events string) error {
	return nil
}
func (m *mockSessionRepo) GetWebhookSubscriptions(ctx context.Context, jid string) ([]waDomain.WebhookSubscription, error) {
	return nil, nil
}
func (m *mockSessionRepo) DeleteWebhookSubscription(ctx context.Context, jid, url string) error {
	return nil
}
func (m *mockSessionRepo) DeleteAllWebhookSubscriptions(ctx context.Context, jid string) error {
	return nil
}
func (m *mockSessionRepo) UpsertSessionStatus(ctx context.Context, phone, state, reason string, banExpiresAt *time.Time) error {
	m.called, m.phone, m.state, m.reason, m.banExp = true, phone, state, reason, banExpiresAt
	return nil
}
func (m *mockSessionRepo) ListSessionStatuses(ctx context.Context) (map[string]waDomain.SessionStatus, error) {
	return nil, nil
}

func newTestLogger(t *testing.T) *customLog.Logger {
	t.Helper()
	l, err := customLog.New(customLog.Config{
		Service: "test", Env: "development", Level: customLog.InfoLevel, Output: customLog.OutputStdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestHandleEventSessionStatus(t *testing.T) {
	const phone = "628999"

	tests := []struct {
		name          string
		evt           any
		wantState     string
		wantEvicted   bool
		wantBanFuture bool
	}{
		{"logged_out_stream", &events.LoggedOut{OnConnect: false}, "logged_out", true, false},
		{"logged_out_connect", &events.LoggedOut{OnConnect: true, Reason: events.ConnectFailureLoggedOut}, "logged_out", true, false},
		{"temporary_ban", &events.TemporaryBan{Code: events.TempBanSentToTooManyPeople, Expire: time.Hour}, "banned", true, true},
		{"connect_failure_loggedout", &events.ConnectFailure{Reason: events.ConnectFailureLoggedOut}, "logged_out", true, false},
		{"connect_failure_transient", &events.ConnectFailure{Reason: events.ConnectFailureServiceUnavailable}, "disconnected", false, false},
		{"stream_replaced", &events.StreamReplaced{}, "disconnected", true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockSessionRepo{}
			h := NewHandler(repo, nil, nil, newTestLogger(t), nil, 1)

			// Seed a live in-memory client so eviction is observable.
			clients.Set(phone, &whatsmeow.Client{})
			t.Cleanup(func() { clients.Delete(phone) })

			h.HandleEvent(phone, tc.evt)

			if !repo.called {
				t.Fatalf("UpsertSessionStatus was not called")
			}
			if repo.state != tc.wantState {
				t.Fatalf("state = %q, want %q", repo.state, tc.wantState)
			}
			if repo.phone != phone {
				t.Fatalf("phone = %q, want %q", repo.phone, phone)
			}
			if tc.wantBanFuture {
				if repo.banExp == nil || !repo.banExp.After(time.Now()) {
					t.Fatalf("ban_expires_at = %v, want a future time", repo.banExp)
				}
			} else if repo.banExp != nil {
				t.Fatalf("ban_expires_at = %v, want nil", repo.banExp)
			}

			evicted := clients.Get(phone) == nil
			if evicted != tc.wantEvicted {
				t.Fatalf("evicted = %v, want %v", evicted, tc.wantEvicted)
			}
		})
	}
}
