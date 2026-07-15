package middleware

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

func newIdempotencyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // one shared in-memory connection
	_, err = db.Exec(`
		CREATE TABLE idempotency_keys (
			phone_number TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			http_status INTEGER,
			response_body TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (phone_number, idempotency_key)
		);`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// newIdempotencyApp wires the middleware in front of a counting handler. The
// handler increments *calls and echoes it, so a replay (handler skipped) leaves
// the counter unchanged.
func newIdempotencyApp(t *testing.T, calls *int) *fiber.App {
	t.Helper()
	logger, err := customLog.New(customLog.Config{Service: "mw-test", Env: "dev", Level: customLog.ErrorLevel, Output: customLog.OutputStdout})
	if err != nil {
		t.Fatal(err)
	}
	mw := &IdempotencyMiddleware{db: newIdempotencyTestDB(t), logger: logger, ttl: time.Hour, pendingTimeout: 30 * time.Second}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(string(contextkeys.PhoneNumber), "628111")
		return c.Next()
	})
	app.Use(mw.Handler())
	app.Post("/send", func(c *fiber.Ctx) error {
		*calls++
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"n": *calls})
	})
	return app
}

func do(t *testing.T, app *fiber.App, key string, body string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, "/send", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Header().Set("Idempotent-Replay", resp.Header.Get("Idempotent-Replay"))
	return rec, string(b)
}

func TestIdempotency_ReplaysSameKeySameBody(t *testing.T) {
	calls := 0
	app := newIdempotencyApp(t, &calls)

	r1, b1 := do(t, app, "k1", `{"to":"1"}`)
	if r1.Code != fiber.StatusOK || b1 != `{"n":1}` {
		t.Fatalf("first: code=%d body=%s", r1.Code, b1)
	}
	r2, b2 := do(t, app, "k1", `{"to":"1"}`)
	if r2.Code != fiber.StatusOK || b2 != `{"n":1}` {
		t.Fatalf("replay: code=%d body=%s (expected the stored {\"n\":1})", r2.Code, b2)
	}
	if r2.Header().Get("Idempotent-Replay") != "true" {
		t.Fatal("replay should set Idempotent-Replay: true")
	}
	if calls != 1 {
		t.Fatalf("handler must run once; ran %d times", calls)
	}
}

func TestIdempotency_SameKeyDifferentBodyIs422(t *testing.T) {
	calls := 0
	app := newIdempotencyApp(t, &calls)

	if r, _ := do(t, app, "k1", `{"to":"1"}`); r.Code != fiber.StatusOK {
		t.Fatalf("first: code=%d", r.Code)
	}
	r, _ := do(t, app, "k1", `{"to":"999"}`) // same key, different body
	if r.Code != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on key reuse with different body, got %d", r.Code)
	}
	if calls != 1 {
		t.Fatalf("handler must not run for the 422; ran %d times", calls)
	}
}

func TestIdempotency_NoKeyNeverDedupes(t *testing.T) {
	calls := 0
	app := newIdempotencyApp(t, &calls)

	for i := 1; i <= 3; i++ {
		r, b := do(t, app, "", `{"to":"1"}`)
		if r.Code != fiber.StatusOK {
			t.Fatalf("call %d: code=%d", i, r.Code)
		}
		if want := fmt.Sprintf(`{"n":%d}`, i); b != want {
			t.Fatalf("call %d: body=%s want=%s", i, b, want)
		}
	}
	if calls != 3 {
		t.Fatalf("no key means no dedup; handler ran %d times, want 3", calls)
	}
}
