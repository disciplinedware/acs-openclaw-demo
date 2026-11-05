package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/bookings"
	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/concierge"
)

const key = "test-key"

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	book, err := bookings.Load("../../testdata/bookings.json", func(string) string { return "" })
	require.NoError(t, err)
	loc, _ := time.LoadLocation("America/Cancun")
	now := time.Date(2026, 11, 4, 14, 0, 0, 0, loc) // CC-1042 arrives that day
	srv := httptest.NewServer(Handler(concierge.New(func() *bookings.Book { return book }, func() time.Time { return now }), key, zap.NewNop()))
	t.Cleanup(srv.Close)
	return srv
}

type bearer struct{ next http.RoundTripper }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+key)
	return b.next.RoundTrip(r)
}

func connect(t *testing.T, srv *httptest.Server) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + MCPPath,
		HTTPClient: &http.Client{Transport: bearer{http.DefaultTransport}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestToolCatalog(t *testing.T) {
	tools, err := connect(t, newServer(t)).ListTools(context.Background(), nil)
	require.NoError(t, err)
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	assert.ElementsMatch(t, []string{ToolFindByPhone, ToolFindByRef, ToolListReservations, ToolGetGuide, ToolGetAccess, ToolListExtras}, names)
}

func TestToolCalls(t *testing.T) {
	cases := []struct {
		name      string
		tool      string
		args      map[string]any
		wantError string
		check     func(t *testing.T, result map[string]any)
	}{
		{
			name: "find_by_phone", tool: ToolFindByPhone, args: map[string]any{"guest_phone": "+447700900100"},
			check: func(t *testing.T, result map[string]any) {
				r := result["reservations"].([]any)[0].(map[string]any)
				assert.Equal(t, "CC-1042", r["reservation_ref"])
				assert.Equal(t, "2026-11-04", r["check_in_date"])
				assert.Equal(t, json.Number("6400"), json.Number(jsonNumber(r["total_price"])))
			},
		},
		{
			name: "find_by_ref_answers_whoever_asks", tool: ToolFindByRef, args: map[string]any{"reservation_ref": "cc-1042"},
			check: func(t *testing.T, result map[string]any) {
				r := result["reservations"].([]any)[0].(map[string]any)
				assert.Equal(t, "Maria Gonzalez", r["guest_name"])
				// The booking's own phone: the Guardian compares it with the sender.
				assert.Equal(t, "+447700900100", r["guest_phone"])
			},
		},
		{name: "find_refuses_a_stranger", tool: ToolFindByPhone, args: map[string]any{"guest_phone": "+447700900199"}, wantError: "/errors/pms.reservation_not_found"},
		{name: "find_refuses_a_national_number", tool: ToolFindByPhone, args: map[string]any{"guest_phone": "07700900100"}, wantError: "/errors/pms.invalid_argument"},
		{
			name: "get_access", tool: ToolGetAccess, args: map[string]any{"listing_id": "2f6a9c1e-5b3d-4c8a-9e10-000000000001"},
			check: func(t *testing.T, result map[string]any) { assert.Equal(t, "4821", result["door_code"]) },
		},
		{name: "get_access_refuses_an_unknown_house", tool: ToolGetAccess, args: map[string]any{"listing_id": "unknown"}, wantError: "/errors/pms.listing_not_found"},
		{
			name: "get_guide", tool: ToolGetGuide, args: map[string]any{"listing_id": "2f6a9c1e-5b3d-4c8a-9e10-000000000002"},
			check: func(t *testing.T, result map[string]any) { assert.Equal(t, "Casita Coral", result["listing_name"]) },
		},
		{name: "extra_list", tool: ToolListExtras, args: map[string]any{}},
	}
	session := connect(t, newServer(t))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &payload))
			if tc.wantError != "" {
				assert.True(t, res.IsError)
				assert.Equal(t, "failed", payload["status"])
				assert.Equal(t, tc.wantError, payload["error"].(map[string]any)["type"])
				return
			}
			assert.False(t, res.IsError, "%v", payload)
			assert.Equal(t, "completed", payload["status"])
			if tc.check == nil {
				extras, ok := payload["result"].([]any)
				require.True(t, ok, "result %v is no list", payload["result"])
				require.Len(t, extras, 2)
				assert.Equal(t, "cenote-tour", extras[0].(map[string]any)["code"])
				assert.Equal(t, json.Number("1800"), json.Number(jsonNumber(extras[0].(map[string]any)["price"])))
				return
			}
			result, ok := payload["result"].(map[string]any)
			require.True(t, ok, "result %v is no object", payload["result"])
			tc.check(t, result)
		})
	}
}

func jsonNumber(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestGuestPermissionsAnswer(t *testing.T) {
	srv := newServer(t)
	post := func(auth, body string) (*http.Response, map[string]any) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+GuestPermissionsPath, bytes.NewBufferString(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var out map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		return resp, out
	}

	resp, out := post("Bearer "+key, `{"guest_phones": ["+447700900100"]}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := out["result"].(map[string]any)
	assert.Equal(t, map[string]any{"guests": false, "others": false}, result["auto_reply"])
	guest := result["guests"].(map[string]any)["+447700900100"].(map[string]any)
	assert.Equal(t, "CC-1042", guest["stay"].(map[string]any)["reservation_ref"])

	resp, out = post("", `{"guest_phones": []}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "/errors/pms.unauthorized", out["type"])

	resp, out = post("Bearer wrong", `{"guest_phones": []}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, out = post("Bearer "+key, `{"phones": []}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "/errors/pms.invalid_argument", out["type"])

	resp, _ = post("Bearer "+key, `{"guest_phones": ["07700900100"]}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestMCPNeedsTheKey(t *testing.T) {
	resp, err := http.Post(newServer(t).URL+MCPPath, "application/json", bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHealthNeedsNoKey(t *testing.T) {
	resp, err := http.Get(newServer(t).URL + HealthPath)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
