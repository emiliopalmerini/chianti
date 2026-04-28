// Package session wraps alexedwards/scs to provide two distinct session
// managers: one for admin login sessions (sqlite-backed) and one for
// short-lived form wizard state (in-memory). The cookie name is supplied by
// the consumer site so the kit stays free of site-specific defaults.
package session

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// NewAdminManager returns a SessionManager for admin logins. Store is
// sqlite-backed against db. Pass secure=true to enable Secure cookies behind
// TLS. cookieName is the site-specific cookie name (e.g. "turno_admin_session").
func NewAdminManager(db *sql.DB, cookieName string, secure bool) *scs.SessionManager {
	m := scs.New()
	m.Store = sqlite3store.New(db)
	m.Lifetime = 24 * time.Hour
	m.IdleTimeout = 2 * time.Hour
	m.Cookie.Name = cookieName
	m.Cookie.HttpOnly = true
	m.Cookie.SameSite = http.SameSiteLaxMode
	m.Cookie.Secure = secure
	m.Cookie.Path = "/"
	return m
}

// NewFormManager returns a SessionManager for short-lived public form state.
// Store is memory-backed; form sessions are short-lived and need not survive
// restarts. cookieName is the site-specific cookie name.
func NewFormManager(cookieName string, secure bool) *scs.SessionManager {
	m := scs.New()
	m.Store = memstore.New()
	m.Lifetime = 2 * time.Hour
	m.IdleTimeout = 30 * time.Minute
	m.Cookie.Name = cookieName
	m.Cookie.HttpOnly = true
	m.Cookie.SameSite = http.SameSiteLaxMode
	m.Cookie.Secure = secure
	m.Cookie.Path = "/"
	return m
}
