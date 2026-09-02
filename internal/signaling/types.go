// Package signaling defines the wire format and authentication used by the
// node's HTTP signaling endpoint.
package signaling

// SessionRequest is the body of POST /session: a client's SDP offer plus a
// Token issued by the panel (see Claims/IssueToken/VerifyToken in token.go),
// which the node verifies against its own node API key.
type SessionRequest struct {
	SDP   string `json:"sdp"`
	Token string `json:"token"`
}

// SessionResponse is the node's reply: its SDP answer.
type SessionResponse struct {
	SDP string `json:"sdp"`
}
