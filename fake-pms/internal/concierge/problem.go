package concierge

// Problem is a refusal in the RFC 9457 problem-details shape. Type is
// `/errors/<code>`; the agent's prompts name the code.
type Problem struct {
	Type      string `json:"type"`
	Status    int    `json:"status"`
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	Retryable bool   `json:"retryable"`
}

func (p *Problem) Error() string { return p.Type + ": " + p.Title }

func problem(code string, status int, title string) *Problem {
	return &Problem{Type: "/errors/" + code, Status: status, Title: title}
}

var (
	errReservationNotFound = problem("pms.reservation_not_found", 404,
		"No current reservation was found for this phone number.")
	errAccessNotAvailable = problem("pms.listing_access_not_available", 409,
		"Access data is given only while a paid reservation of this house is open, from a few hours before its check-in time until its check-out time.")
	errListingNotFound = problem("pms.listing_not_found", 404,
		"No listing has this listing_id.")
)

// InvalidArgument refuses a call whose argument has the wrong form.
func InvalidArgument(detail string) *Problem {
	p := problem("pms.invalid_argument", 400, "An argument is not valid.")
	p.Detail = detail
	return p
}
