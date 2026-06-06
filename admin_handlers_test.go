package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadAdminTemplates populates the package-level templates var for handler
// tests, mirroring what registerAdminUI does at startup.
func loadAdminTemplates(t *testing.T) {
	t.Helper()
	tmpls, err := loadTemplates()
	require.NoError(t, err)
	templates = tmpls
}

func TestLoadTemplates(t *testing.T) {
	tmpls, err := loadTemplates()
	require.NoError(t, err)
	require.NotNil(t, tmpls)

	for _, view := range []string{"dashboard.html", "map.html", "trips.html", "users.html", "vehicles.html"} {
		assert.Contains(t, tmpls.admin, view, "admin view %q should be parsed", view)
	}
	assert.Contains(t, tmpls.public, "login.html")
}

func TestAdminHandlersRenderOK(t *testing.T) {
	loadAdminTemplates(t)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		path    string
		want    string
	}{
		{"dashboard", AdminDashboardHandler, "/admin/dashboard", "Bus 001"},
		{"vehicles", AdminVehiclesHandler, "/admin/vehicles", "Bus 001"},
		{"users", AdminUsersHandler, "/admin/users", "Chaitanya K"},
		{"trips", AdminTripsHandler, "/admin/trips", "Route A"},
		{"map", AdminMapHandler, "/admin/map", "Live Map"},
		{"login", AdminLoginHandler, "/admin/login", "Welcome"},
		{"signup", AdminSignupHandler, "/admin/signup", "Create Account"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

// TestExecuteTemplateUnknownNames verifies the dispatcher rejects unknown view
// names instead of silently falling back to a default template.
func TestExecuteTemplateUnknownNames(t *testing.T) {
	loadAdminTemplates(t)

	t.Run("unknown public", func(t *testing.T) {
		err := templates.ExecuteTemplate(&bytes.Buffer{}, "does-not-exist.html", map[string]interface{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown public template")
	})

	t.Run("unknown admin", func(t *testing.T) {
		data := map[string]interface{}{adminTemplateKey: "ghost.html"}
		err := templates.ExecuteTemplate(&bytes.Buffer{}, "base.html", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown admin template")
	})
}

// TestRenderWritesCleanErrorResponse verifies a render failure produces a 500
// rather than a partially written 200 body.
func TestRenderWritesCleanErrorResponse(t *testing.T) {
	loadAdminTemplates(t)

	rec := httptest.NewRecorder()
	// "/" has no registered template, so render must fail cleanly.
	render(rec, "/", "missing.html", map[string]interface{}{})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal server error")
}

func TestRegisterAdminUI(t *testing.T) {
	mux := http.NewServeMux()
	require.NoError(t, registerAdminUI(mux))

	t.Run("admin route is served", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("static asset is served from embedded fs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/js/admin.js", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotEmpty(t, rec.Body.String())
	})
}
