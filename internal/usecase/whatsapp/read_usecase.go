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
