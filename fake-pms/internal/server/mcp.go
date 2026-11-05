package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/concierge"
)

// The five guest tools and the host's one. OpenClaw names each `pms__<name with dots as
// hyphens>`, the form the Guardian's rules match.
const (
	ToolFindByPhone      = "pms.reservation.find_by_phone"
	ToolFindByRef        = "pms.reservation.find_by_ref"
	ToolGetGuide         = "pms.listing.get_guide"
	ToolGetAccess        = "pms.listing.get_access"
	ToolListExtras       = "pms.extra.list"
	ToolListReservations = "pms.host.list_reservations"
)

type findByPhoneIn struct {
	GuestPhone string `json:"guest_phone" jsonschema:"the guest's phone in E.164, with its + country code"`
}

type findByRefIn struct {
	ReservationRef string `json:"reservation_ref" jsonschema:"the reservation_ref, or the channel_reservation_id a guest may quote"`
}

type getGuideIn struct {
	ListingID string `json:"listing_id" jsonschema:"the listing_id of a reservation"`
}

type getAccessIn struct {
	ListingID string `json:"listing_id" jsonschema:"the listing_id of a reservation"`
}

type noIn struct{}

// NewMCPServer declares the tools with the descriptions the agent reads.
func NewMCPServer(c *concierge.Concierge) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "pms", Version: "1.0.0"}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolGetGuide, Annotations: readOnly,
		Description: "The public information of a house, by the listing_id of a reservation: address and coordinates, check-in and check-out times, the guest guide (parking, beach, rules). Never the door code or Wi-Fi.",
	}, handle(func(in getGuideIn) (any, error) { return c.GetGuide(in.ListingID) }))
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolListExtras, Annotations: readOnly,
		Description: "The extras this business sells (airport transfer, tours) with their codes and prices.",
	}, handle(func(noIn) (any, error) { return c.ListExtras(), nil }))
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolFindByPhone, Annotations: readOnly,
		Description: "Every current reservation of the guest with this phone number, in this business: its reservation_ref, the channel_reservation_id a guest may quote (the Airbnb or Booking.com code), its status (confirmed, or checked_in once the guest has arrived), the business and listing, the adults and each child's age (child_ages), dates and times, total_price and currency of the stay and whether it is_paid. A guest may hold several.",
	}, handle(func(in findByPhoneIn) (any, error) { return c.FindByPhone(in.GuestPhone) }))
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolFindByRef, Annotations: readOnly,
		Description: "The current reservation with this reference, in this business, its reservation_ref, the channel_reservation_id a guest may quote (the Airbnb or Booking.com code), its status (confirmed, or checked_in once the guest has arrived), the business and listing, the adults and each child's age (child_ages), dates and times, total_price and currency of the stay and whether it is_paid.",
	}, handle(func(in findByRefIn) (any, error) { return c.FindByRef(in.ReservationRef) }))
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolGetAccess, Annotations: readOnly,
		Description: "The door code and Wi-Fi of a house, by the listing_id of a reservation. Given only while a paid reservation of this house is open, from a few hours before its check-in time (the business sets how many) until its check-out time.",
	}, handle(func(in getAccessIn) (any, error) { return c.GetAccess(in.ListingID) }))
	mcp.AddTool(s, &mcp.Tool{
		Name: ToolListReservations, Annotations: readOnly,
		Description: "Every current reservation of this business, for the host: its reservation_ref, guest name and phone, listing and dates.",
	}, handle(func(noIn) (any, error) { return c.ListReservations(), nil }))
	return s
}

// handle renders a concierge answer: {"status":"completed","result":...}, or
// a tool error {"status":"failed","error":<problem>} with isError set.
func handle[In any](f func(In) (any, error)) mcp.ToolHandlerFor[In, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		result, err := f(in)
		if err != nil {
			var p *concierge.Problem
			if !errors.As(err, &p) {
				return nil, nil, err
			}
			r := toolResult(map[string]any{"status": "failed", "error": p})
			r.IsError = true
			return r, nil, nil
		}
		return toolResult(map[string]any{"status": "completed", "result": result}), nil, nil
	}
}

func toolResult(payload map[string]any) *mcp.CallToolResult {
	body, _ := json.Marshal(payload)
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(body)}},
		StructuredContent: payload,
	}
}
