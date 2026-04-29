package httptest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emiliopalmerini/chianti/platform/httpx"
	chttptest "github.com/emiliopalmerini/chianti/platform/httpx/httptest"
)

func TestCSRFBypassInjectsNoOpField(t *testing.T) {
	var got string
	mw := chttptest.CSRFBypass()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := httpx.CSRFFieldFromContext(r.Context())
		if f == nil {
			t.Fatal("CSRFField nil after bypass")
		}
		got = string(f(r))
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got != "" {
		t.Fatalf("CSRFField = %q, want empty", got)
	}
}
