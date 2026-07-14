package whatsapp_usecase

import (
	"context"
	"sort"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

const (
	defaultContactPageLimit = 100
	maxContactPageLimit     = 500
)

// ListContacts returns a page of the account's locally-synced contacts. The
// list reflects whatsmeow's synced address book, which populates after pairing
// and as the account is used — an empty or partial result is not an error, so
// the response carries a metadata note rather than a 404. Reads come straight
// from the local store (no network, no rate budget). Pagination is applied here
// over a JID-sorted slice so results are stable across calls.
func (uc *WhatsappMessageUsecase) ListContacts(
	ctx context.Context,
	traceID, phoneNumber string,
	limit, offset int,
) (*waDomain.ContactListResponse, error) {
	items, err := uc.whatsappManager.ListContacts(ctx, traceID, phoneNumber)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool { return items[i].JID < items[j].JID })
	total := len(items)

	if limit <= 0 {
		limit = defaultContactPageLimit
	}
	if limit > maxContactPageLimit {
		limit = maxContactPageLimit
	}
	if offset < 0 {
		offset = 0
	}

	start := min(offset, total)
	end := min(start+limit, total)
	page := items[start:end]

	return &waDomain.ContactListResponse{
		Contacts: page,
		Count:    len(page),
		Total:    total,
		Note:     "Reflects the account's locally-synced contacts; may be incomplete until history sync completes.",
	}, nil
}

// ListGroups returns the account's joined groups. Unlike contacts, this hits the
// WhatsApp server (GetJoinedGroups), so it is served through the shared TTL
// cache + per-account read budget: repeat polls hit the cache for free, and a
// cache miss spends one budget token (429 when exhausted). WhatsApp returns the
// full set in one call, so the result is not paginated.
func (uc *WhatsappMessageUsecase) ListGroups(
	ctx context.Context,
	traceID, phoneNumber string,
) (*waDomain.GroupListResponse, error) {
	return queryWithBudget(uc, ctx, phoneNumber, "groups:"+phoneNumber,
		func() (*waDomain.GroupListResponse, error) {
			items, err := uc.whatsappManager.ListGroups(ctx, traceID, phoneNumber)
			if err != nil {
				return nil, err
			}
			sort.Slice(items, func(i, j int) bool { return items[i].JID < items[j].JID })
			return &waDomain.GroupListResponse{Groups: items, Count: len(items)}, nil
		})
}
