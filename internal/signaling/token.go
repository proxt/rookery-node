package signaling

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrTokenMalformed is returned when a token isn't in payload.signature form.
	ErrTokenMalformed = errors.New("signaling: malformed token")
	// ErrTokenSignature is returned when a token's signature doesn't verify.
	ErrTokenSignature = errors.New("signaling: invalid token signature")
	// ErrTokenExpired is returned when a token's claims have expired.
	ErrTokenExpired = errors.New("signaling: token expired")
)

// Claims is what a panel-issued session token asserts: which user is
// connecting, which node the token is scoped to, and until when it's valid.
type Claims struct {
	UserID string `json:"uid"`
	NodeID string `json:"nid"`
	Expiry int64  `json:"exp"` // Unix seconds
}

// IssueToken signs claims with nodeAPIKey, producing the opaque string a
// client presents in SessionRequest.Token. nodeAPIKey is the shared secret
// established when the panel registered the node claims.NodeID names — only
// that node can verify a token issued with its key.
func IssueToken(nodeAPIKey []byte, claims Claims) (string, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("signaling: marshal claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(data)
	return payload + "." + signPayload(nodeAPIKey, payload), nil
}

// VerifyToken checks token's signature against nodeAPIKey and that it hasn't
// expired as of now, returning the claims it asserts.
func VerifyToken(nodeAPIKey []byte, token string, now time.Time) (Claims, error) {
	i := len(token)
	for i > 0 && token[i-1] != '.' {
		i--
	}
	if i == 0 {
		return Claims{}, ErrTokenMalformed
	}
	payload, sigHex := token[:i-1], token[i:]

	want := signPayload(nodeAPIKey, payload)
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return Claims{}, fmt.Errorf("signaling: encode expected signature: %w", err)
	}
	gotBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: not valid hex", ErrTokenSignature)
	}
	if !hmac.Equal(wantBytes, gotBytes) {
		return Claims{}, ErrTokenSignature
	}

	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenMalformed, err)
	}
	var claims Claims
	if err := json.Unmarshal(data, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenMalformed, err)
	}

	if now.After(time.Unix(claims.Expiry, 0)) {
		return Claims{}, ErrTokenExpired
	}
	return claims, nil
}

func signPayload(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
