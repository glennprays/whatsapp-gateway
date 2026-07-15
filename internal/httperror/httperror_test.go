package httperror

import (
	"encoding/json"
	"errors"
	"testing"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
)

// TestAPIError_JSONShape locks the wire contract: an error body must serialize as
// the documented ErrorResponse {"error","code"} (openapi.yaml), NOT the Go field
// names {"Status","Message"}. Clients (the SDK) decode "error"/"code"; a regression
// to tagless fields would silently blank every error message on the wire.
func TestAPIError_JSONShape(t *testing.T) {
	b, err := json.Marshal(APIError{Status: 404, Message: "group not found"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["error"] != "group not found" {
		t.Errorf(`want "error":"group not found", got %v (body=%s)`, m["error"], b)
	}
	if code, _ := m["code"].(float64); int(code) != 404 {
		t.Errorf(`want "code":404, got %v (body=%s)`, m["code"], b)
	}
	if _, leaked := m["Message"]; leaked {
		t.Errorf("tagless Go field leaked to the wire: %s", b)
	}
}

// TestFromError_MapsDomainStatus spot-checks the domain-error → HTTP-status mapping
// that feeds the code field.
func TestFromError_MapsDomainStatus(t *testing.T) {
	cases := map[error]int{
		errDomain.NewError(errDomain.ErrNotFound, errors.New("x")):        404,
		errDomain.NewError(errDomain.ErrForbidden, errors.New("x")):       403,
		errDomain.NewError(errDomain.ErrTooManyRequests, errors.New("x")): 429,
		errDomain.NewError(errDomain.ErrGone, errors.New("x")):            410,
	}
	for in, want := range cases {
		if got := FromError(in); got.Status != want {
			t.Errorf("FromError(%v).Status = %d, want %d", in, got.Status, want)
		}
	}
}
