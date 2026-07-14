package whatsapp

import (
	"context"
	"net/http"
	"testing"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
)

// stubWebhookClient embeds the Client interface so only the two methods the
// webhook-set path touches need implementing; loggedIn drives LoginStatus and
// setCalled records whether the write reached the client (it must not on a
// validation failure).
type stubWebhookClient struct {
	Client
	loggedIn  bool
	setCalled bool
}

func (s *stubWebhookClient) LoginStatus(traceID, phoneNumber string) (bool, error) {
	return s.loggedIn, nil
}
func (s *stubWebhookClient) SetWebhookSubscription(ctx context.Context, traceID, phoneNumber string, webhook *waDomain.Webhook) error {
	s.setCalled = true
	return nil
}

func newManagerWithClient(t *testing.T, cli Client) *manager {
	t.Helper()
	return &manager{
		Client: cli,
		Cipher: cipherx.NewCipher("0123456789abcdef0123456789abcdef"),
		Logger: newTestLogger(t),
	}
}

func TestSetWebhookSubscriptionRejectsUnknownEvent(t *testing.T) {
	cli := &stubWebhookClient{loggedIn: true}
	m := newManagerWithClient(t, cli)

	err := m.SetWebhookSubscription(context.Background(), "tid", "628111",
		&waDomain.Webhook{Url: "https://ok.example/hook", Events: []string{"message.sent", "bogus.event"}})
	if err == nil {
		t.Fatal("expected an error for an unknown event type")
	}
	if httpErr := httperror.FromError(err); httpErr.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", httpErr.Status)
	}
	if cli.setCalled {
		t.Fatal("client write must not run when event validation fails")
	}
}

func TestSetWebhookSubscriptionNotLoggedIn(t *testing.T) {
	cli := &stubWebhookClient{loggedIn: false}
	m := newManagerWithClient(t, cli)

	err := m.SetWebhookSubscription(context.Background(), "tid", "628111",
		&waDomain.Webhook{Url: "https://ok.example/hook"})
	if err == nil {
		t.Fatal("expected a not-logged-in error")
	}
	if httpErr := httperror.FromError(err); httpErr.Status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", httpErr.Status)
	}
	if cli.setCalled {
		t.Fatal("client write must not run when not logged in")
	}
}
