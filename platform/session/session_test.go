package session_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/emiliopalmerini/chianti/platform/database"
	"github.com/emiliopalmerini/chianti/platform/session"
)

func TestAdminManagerSetsCookieName(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := session.NewAdminManager(db.DB, "x_admin", false)
	if m.Cookie.Name != "x_admin" {
		t.Errorf("cookie name = %q, want %q", m.Cookie.Name, "x_admin")
	}
}

func TestFormManagerSetsCookieName(t *testing.T) {
	m := session.NewFormManager("x_form", false)
	if m.Cookie.Name != "x_form" {
		t.Errorf("cookie name = %q, want %q", m.Cookie.Name, "x_form")
	}
}

func TestAdminManagerHasExpectedDefaults(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := session.NewAdminManager(db.DB, "x", false)
	if m.Lifetime != 24*time.Hour {
		t.Errorf("lifetime = %v, want 24h", m.Lifetime)
	}
	if m.IdleTimeout != 2*time.Hour {
		t.Errorf("idle = %v, want 2h", m.IdleTimeout)
	}
	if !m.Cookie.HttpOnly {
		t.Error("cookie not HttpOnly")
	}
	if m.Cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("samesite = %v, want lax", m.Cookie.SameSite)
	}
	if m.Cookie.Path != "/" {
		t.Errorf("path = %q, want /", m.Cookie.Path)
	}
	if m.Cookie.Secure {
		t.Error("Secure must follow the parameter (false here)")
	}
}

func TestFormManagerHasExpectedDefaults(t *testing.T) {
	m := session.NewFormManager("x", true)
	if m.Lifetime != 2*time.Hour {
		t.Errorf("lifetime = %v, want 2h", m.Lifetime)
	}
	if m.IdleTimeout != 30*time.Minute {
		t.Errorf("idle = %v, want 30m", m.IdleTimeout)
	}
	if !m.Cookie.HttpOnly {
		t.Error("cookie not HttpOnly")
	}
	if m.Cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("samesite = %v, want lax", m.Cookie.SameSite)
	}
	if !m.Cookie.Secure {
		t.Error("Secure must follow the parameter (true here)")
	}
}

func TestCookieIsolation(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	admin := session.NewAdminManager(db.DB, "a_admin", false)
	form := session.NewFormManager("a_form", false)
	if admin.Cookie.Name == form.Cookie.Name {
		t.Errorf("admin and form share cookie name %q", admin.Cookie.Name)
	}
}
