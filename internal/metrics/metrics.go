// Package metrics emits a small, hand-rolled Prometheus text exposition. It
// deliberately avoids prometheus/client_golang: the two counters and one gauge
// here are trivial to render by hand, and pulling in the client library (plus
// its transitive deps) for that is not worth it.
//
// Cardinality is bounded by construction: every label value is a fixed internal
// string (message type, mode, result, event, session state). A phone number is
// NEVER used as a label — per-account detail lives on /admin/sessions instead.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

const namespace = "whatsapp_gateway"

// counter is a label-set -> value map. The key is the pre-rendered label block
// (e.g. `{type="text",mode="direct",result="success"}`).
type counter struct {
	mu   sync.Mutex
	vals map[string]int64
}

func (c *counter) inc(labels string) {
	c.mu.Lock()
	if c.vals == nil {
		c.vals = make(map[string]int64)
	}
	c.vals[labels]++
	c.mu.Unlock()
}

func (c *counter) snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.vals))
	for k, v := range c.vals {
		out[k] = v
	}
	return out
}

var (
	messages counter
	webhooks counter

	sessionsMu sync.RWMutex
	sessionsFn func() map[string]int
)

func result(err error) string {
	if err != nil {
		return "failure"
	}
	return "success"
}

// RecordSend counts one outbound send attempt (mode: direct|queue).
func RecordSend(msgType, mode string, err error) {
	messages.inc(fmt.Sprintf(`{type=%q,mode=%q,result=%q}`, msgType, mode, result(err)))
}

// RecordWebhook counts one webhook delivery attempt (mode: direct|queue).
func RecordWebhook(mode, event string, err error) {
	webhooks.inc(fmt.Sprintf(`{result=%q,mode=%q,event=%q}`, result(err), mode, event))
}

// SetSessionsSource registers the scrape-time source for the sessions gauge. The
// gauge is recomputed on every scrape from this closure.
//
// ponytail: gauge recomputed per scrape; if SessionInventory ever gets expensive,
// cache it behind READ_QUERY TTL.
func SetSessionsSource(fn func() map[string]int) {
	sessionsMu.Lock()
	sessionsFn = fn
	sessionsMu.Unlock()
}

// Handler serves the exposition as text/plain.
func Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return c.SendString(render())
	}
}

func render() string {
	var b strings.Builder
	writeCounter(&b, "messages_total", "Total outbound message send attempts.", messages.snapshot())
	writeCounter(&b, "webhook_deliveries_total", "Total webhook delivery attempts.", webhooks.snapshot())
	writeSessions(&b)
	return b.String()
}

func writeCounter(b *strings.Builder, name, help string, vals map[string]int64) {
	full := namespace + "_" + name
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n", full, help, full)
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "%s%s %d\n", full, k, vals[k])
	}
}

func writeSessions(b *strings.Builder) {
	full := namespace + "_sessions"
	fmt.Fprintf(b, "# HELP %s Current WhatsApp sessions by state (per instance).\n# TYPE %s gauge\n", full, full)
	sessionsMu.RLock()
	fn := sessionsFn
	sessionsMu.RUnlock()
	if fn == nil {
		return
	}
	states := fn()
	keys := make([]string, 0, len(states))
	for s := range states {
		keys = append(keys, s)
	}
	sort.Strings(keys)
	for _, s := range keys {
		fmt.Fprintf(b, "%s{state=%q} %d\n", full, s, states[s])
	}
}
