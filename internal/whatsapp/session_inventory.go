package whatsapp

import (
	"context"
	"os"
	"sort"
	"time"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

// clientState is the minimal projection of an in-memory whatsmeow client needed
// to derive an honest session state, so the merge logic is testable without a
// real *whatsmeow.Client.
type clientState struct {
	paired    bool // Store.ID != nil
	connected bool
	loggedIn  bool
}

// deriveItem computes the honest state for one account from the three sources,
// applying precedence: a persisted dead-session (logged_out, or unexpired ban)
// wins over a lingering in-memory client; otherwise the live client is the most
// authoritative; otherwise persisted/device rows report as store-sourced.
func deriveItem(phone string, inMem map[string]clientState, devices map[string]time.Time, status map[string]waDomain.SessionStatus, now time.Time) waDomain.SessionInventoryItem {
	item := waDomain.SessionInventoryItem{PhoneMasked: MaskedPhoneNumber(phone)}

	st, hasStatus := status[phone]
	banUnexpired := hasStatus && st.State == "banned" && (st.BanExpiresAt == nil || st.BanExpiresAt.After(now))

	// Persisted dead-session wins over any in-memory client.
	if hasStatus && (st.State == "logged_out" || banUnexpired) {
		item.State = st.State
		item.Source = "store"
		item.Reason = st.Reason
		item.LastSeen = st.UpdatedAt
		if banUnexpired && st.BanExpiresAt != nil {
			item.BanExpiresAt = st.BanExpiresAt
		}
		return item
	}

	// Live in-memory client is the most authoritative for a still-active session.
	if cs, ok := inMem[phone]; ok {
		item.Source = "in-memory"
		item.LastSeen = now
		switch {
		case !cs.paired:
			item.State = "never_paired"
		case cs.connected && cs.loggedIn:
			item.State = "connected"
		default:
			item.State = "disconnected"
		}
		return item
	}

	// No live client: fall back to persisted status, then to a bare device row.
	if hasStatus {
		item.Source = "store"
		item.LastSeen = st.UpdatedAt
		if st.State == "banned" {
			// Ban present but expired (unexpired handled above): downgrade.
			item.State = "disconnected"
			item.Reason = "ban_expired"
		} else {
			item.State = st.State
			item.Reason = st.Reason
		}
		return item
	}

	item.State = "disconnected"
	item.Source = "store"
	item.LastSeen = devices[phone]
	return item
}

// mergeInventory folds the three sources into a deterministic, phone-sorted list
// of honest per-account states. Phone numbers are masked in the output.
func mergeInventory(inMem map[string]clientState, devices map[string]time.Time, status map[string]waDomain.SessionStatus, now time.Time) []waDomain.SessionInventoryItem {
	phones := make(map[string]struct{}, len(inMem)+len(devices)+len(status))
	for p := range inMem {
		phones[p] = struct{}{}
	}
	for p := range devices {
		phones[p] = struct{}{}
	}
	for p := range status {
		phones[p] = struct{}{}
	}

	sorted := make([]string, 0, len(phones))
	for p := range phones {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	items := make([]waDomain.SessionInventoryItem, 0, len(sorted))
	for _, p := range sorted {
		items = append(items, deriveItem(p, inMem, devices, status, now))
	}
	return items
}

// sessionSources gathers the three inputs to the merge: the in-memory client set
// (highest fidelity), the persisted device rows (a paired-but-offline signal),
// and the persisted lifecycle statuses (authoritative for dead sessions).
func (c *client) sessionSources(ctx context.Context) (map[string]clientState, map[string]time.Time, map[string]waDomain.SessionStatus, error) {
	status, err := c.repository.ListSessionStatuses(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	devices := map[string]time.Time{}
	deviceRows, err := c.container.GetAllDevices(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	for _, d := range deviceRows {
		if d.ID == nil {
			continue
		}
		devices[WhatsappDecomposeJID(d.ID.User)] = now
	}

	inMem := map[string]clientState{}
	for phone, cli := range clients.GetAll() {
		if cli == nil {
			continue
		}
		inMem[phone] = clientState{
			paired:    cli.Store != nil && cli.Store.ID != nil,
			connected: cli.IsConnected(),
			loggedIn:  cli.IsLoggedIn(),
		}
	}
	return inMem, devices, status, nil
}

// SessionInventory merges all sources into per-account items (masked, sorted).
func (c *client) SessionInventory(ctx context.Context) ([]waDomain.SessionInventoryItem, error) {
	inMem, devices, status, err := c.sessionSources(ctx)
	if err != nil {
		return nil, err
	}
	return mergeInventory(inMem, devices, status, time.Now()), nil
}

// GetOneSession returns the merged item for one bare phone, or (nil, nil) when
// the phone matches no in-memory client, device row, nor status row.
func (c *client) GetOneSession(ctx context.Context, phone string) (*waDomain.SessionInventoryItem, error) {
	inMem, devices, status, err := c.sessionSources(ctx)
	if err != nil {
		return nil, err
	}
	_, a := inMem[phone]
	_, b := devices[phone]
	_, d := status[phone]
	if !a && !b && !d {
		return nil, nil
	}
	item := deriveItem(phone, inMem, devices, status, time.Now())
	return &item, nil
}

// SessionInventory wraps the client merge with the per-instance envelope
// (hostname + count). The view is per-instance by design: a device with no live
// client here may be live on another node.
func (m *manager) SessionInventory(ctx context.Context, traceID string) (*waDomain.SessionInventory, error) {
	items, err := m.Client.SessionInventory(ctx)
	if err != nil {
		return nil, err
	}
	return &waDomain.SessionInventory{
		Instance: Hostname(),
		Count:    len(items),
		Sessions: items,
	}, nil
}

// GetOneSession returns a single account's merged state, or (nil, nil) if absent.
func (m *manager) GetOneSession(ctx context.Context, traceID, phone string) (*waDomain.SessionInventoryItem, error) {
	return m.Client.GetOneSession(ctx, phone)
}

// Hostname returns this instance's hostname (falling back to "unknown"), used
// as the per-instance identifier in admin/session responses.
func Hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
