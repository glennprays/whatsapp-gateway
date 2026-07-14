package metrics

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderCountersAndGauge(t *testing.T) {
	RecordSend("text", "direct", nil)
	RecordSend("text", "direct", nil)
	RecordSend("image", "queue", errors.New("boom"))
	RecordWebhook("direct", "message.incoming", nil)

	SetSessionsSource(func() map[string]int {
		return map[string]int{"connected": 2, "banned": 1}
	})

	out := render()

	wants := []string{
		"# TYPE whatsapp_gateway_messages_total counter",
		`whatsapp_gateway_messages_total{type="text",mode="direct",result="success"} 2`,
		`whatsapp_gateway_messages_total{type="image",mode="queue",result="failure"} 1`,
		`whatsapp_gateway_webhook_deliveries_total{result="success",mode="direct",event="message.incoming"} 1`,
		"# TYPE whatsapp_gateway_sessions gauge",
		`whatsapp_gateway_sessions{state="connected"} 2`,
		`whatsapp_gateway_sessions{state="banned"} 1`,
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Fatalf("exposition missing %q\n---\n%s", w, out)
		}
	}

	// Cardinality guard: a phone number must never appear as a label value.
	if strings.Contains(out, "628") || strings.Contains(out, "phone") {
		t.Fatalf("exposition leaked a phone-shaped label:\n%s", out)
	}
}
