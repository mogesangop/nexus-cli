package nexus

import "fmt"

// User represents a Nexus security user.
type User struct {
	UserID       string   `json:"userID"`
	FirstName    string   `json:"firstName,omitempty"`
	LastName     string   `json:"lastName,omitempty"`
	EmailAddress string   `json:"emailAddress,omitempty"`
	Status       string   `json:"status,omitempty"`
	Roles        []string `json:"roles,omitempty"`
}

// CreateUser creates a user. The password is NOT set here; Nexus 3.x separates
// user creation from password setting. Use SetPassword afterwards.
// Endpoint: POST /security/users.
//
// NOTE: Field names should be verified against the target Nexus 3.76 Swagger.
func (c *Client) CreateUser(u *User) (*User, error) {
	body := map[string]any{
		"userID":       u.UserID,
		"firstName":    u.FirstName,
		"lastName":     u.LastName,
		"emailAddress": u.EmailAddress,
		"status":       u.Status,
		"roles":        u.Roles,
	}
	var out User
	if err := c.post("/security/users", body, &out); err != nil {
		return nil, fmt.Errorf("create user %s: %w", u.UserID, err)
	}
	return &out, nil
}

// GetUser fetches a user by id. Returns an *APIError with Status 404 (see
// IsNotFound) when it does not exist.
// Endpoint: GET /security/users/{id}.
func (c *Client) GetUser(userID string) (*User, error) {
	var out User
	if err := c.get("/security/users/"+userID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetPassword sets the password for a user. This is the admin-set path
// (distinct from the user self-change path that requires the current password).
// The password is transmitted only in the request body to Nexus; it never
// enters logs, audit records, or error messages.
// Endpoint: PUT /security/users/{id}/change-password.
//
// NOTE: The exact request body shape varies across Nexus minor versions
// (candidate shapes: {"newPassword":"..."} or a raw base64-encoded string).
// Verify against the target Nexus 3.76 Swagger before relying on this.
func (c *Client) SetPassword(userID, password string) error {
	body := map[string]any{
		"newPassword": password,
	}
	if err := c.put("/security/users/"+userID+"/change-password", body); err != nil {
		return fmt.Errorf("set password for %s: %w", userID, err)
	}
	return nil
}
