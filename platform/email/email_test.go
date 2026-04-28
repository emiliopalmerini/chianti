package email_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emiliopalmerini/chianti/platform/email"
)

type capture struct {
	last email.Message
}

func (c *capture) Send(_ context.Context, msg email.Message) error {
	c.last = msg
	return nil
}

func TestDevOverrideRedirects(t *testing.T) {
	inner := &capture{}
	wrapped := email.NewDevOverride(inner, "dev@example.com")
	err := wrapped.Send(context.Background(), email.Message{
		To:      []string{"customer@example.com"},
		Subject: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.last.To) != 1 || inner.last.To[0] != "dev@example.com" {
		t.Errorf("to = %v, want [dev@example.com]", inner.last.To)
	}
}

func TestResendSenderRequiresKey(t *testing.T) {
	if _, err := email.NewResendSender("", "x@y"); err == nil {
		t.Error("expected error for empty api key")
	}
	if _, err := email.NewResendSender("k", ""); err == nil {
		t.Error("expected error for empty from")
	}
}

func TestNoopReturnsNil(t *testing.T) {
	s := email.NewNoop(nil) // nil logger, falls back to slog.Default()
	err := s.Send(context.Background(), email.Message{
		To:      []string{"x@y"},
		Subject: "test",
	})
	if err != nil {
		t.Errorf("noop returned error: %v", err)
	}
}

func TestResendSenderErrorsOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	sender, err := email.NewResendSenderWithEndpoint("key", "from@x", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Send(context.Background(), email.Message{
		To:      []string{"to@y"},
		Subject: "s",
		HTML:    "h",
	})
	if err == nil {
		t.Fatal("expected error on 4xx")
	}
	if !contains(err.Error(), "400") || !contains(err.Error(), "bad request") {
		t.Errorf("error message missing status or body: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
