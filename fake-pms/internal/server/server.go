// Package server exposes the concierge over HTTP: the MCP server `pms` at
// /mcp (streamable HTTP) and the Guardian's permissions read at
// /api/actions/pms.guardian.guest_permissions. Both take the same bearer key.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/concierge"
)

const (
	HealthPath           = "/health"
	MCPPath              = "/mcp"
	GuestPermissionsPath = "/api/actions/pms.guardian.guest_permissions"
	// maxBodyBytes bounds a permissions request; it carries a few phones.
	maxBodyBytes = 64 << 10
)

// Handler routes both surfaces behind the bearer key, and an open health
// check that answers 200 once the bookings file is loaded.
func Handler(c *concierge.Concierge, apiKey string, log *zap.Logger) http.Handler {
	mux := http.NewServeMux()
	mcpServer := NewMCPServer(c)
	mux.Handle(MCPPath, mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	))
	mux.Handle("POST "+GuestPermissionsPath, guestPermissionsHandler(c, log))
	open := http.NewServeMux()
	open.HandleFunc("GET "+HealthPath, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	open.Handle("/", requireBearer(apiKey, log, mux))
	return open
}

func requireBearer(apiKey string, log *zap.Logger, next http.Handler) http.Handler {
	want := []byte("Bearer " + apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			log.Warn("request refused: missing or wrong bearer key",
				zap.String("path", r.URL.Path), zap.String("remote", r.RemoteAddr))
			writeProblem(w, &concierge.Problem{
				Type: "/errors/pms.unauthorized", Status: http.StatusUnauthorized,
				Title: "The Authorization header does not carry this server's bearer key.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type guestPermissionsRequest struct {
	GuestPhones []string `json:"guest_phones"`
}

func guestPermissionsHandler(c *concierge.Concierge, log *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		dec.DisallowUnknownFields()
		var req guestPermissionsRequest
		if err := dec.Decode(&req); err != nil || req.GuestPhones == nil {
			detail := "the body is {\"guest_phones\": [\"+52...\"]}"
			if err != nil {
				detail = err.Error()
			}
			log.Warn("permissions read refused: invalid body", zap.String("detail", detail))
			writeProblem(w, concierge.InvalidArgument(detail))
			return
		}
		for _, p := range req.GuestPhones {
			if !strings.HasPrefix(p, "+") {
				log.Warn("permissions read refused: phone without country code", zap.String("phone", p))
				writeProblem(w, concierge.InvalidArgument("guest_phones holds a number without its + country code"))
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"result": c.GuestPermissions(req.GuestPhones)})
	})
}

func writeProblem(w http.ResponseWriter, p *concierge.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
