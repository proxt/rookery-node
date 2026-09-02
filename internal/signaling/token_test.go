package signaling

import (
	"errors"
	"testing"
	"time"
)

func TestIssueVerifyTokenRoundTrip(t *testing.T) {
	key := []byte("node-api-key")
	claims := Claims{SubscriptionID: "sub1", NodeID: "node1", Expiry: time.Now().Add(time.Hour).Unix()}

	token, err := IssueToken(key, claims)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	got, err := VerifyToken(key, token, time.Now())
	if err != nil {
		t.Fatalf("VerifyToken() error = %v, want nil", err)
	}
	if got != claims {
		t.Fatalf("VerifyToken() = %+v, want %+v", got, claims)
	}
}

func TestVerifyTokenRejectsExpired(t *testing.T) {
	key := []byte("node-api-key")
	claims := Claims{SubscriptionID: "sub1", NodeID: "node1", Expiry: time.Now().Add(-time.Minute).Unix()}

	token, err := IssueToken(key, claims)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	if _, err := VerifyToken(key, token, time.Now()); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("VerifyToken() error = %v, want ErrTokenExpired", err)
	}
}

func TestVerifyTokenRejectsWrongKey(t *testing.T) {
	claims := Claims{SubscriptionID: "sub1", NodeID: "node1", Expiry: time.Now().Add(time.Hour).Unix()}
	token, err := IssueToken([]byte("key-a"), claims)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	if _, err := VerifyToken([]byte("key-b"), token, time.Now()); !errors.Is(err, ErrTokenSignature) {
		t.Fatalf("VerifyToken() error = %v, want ErrTokenSignature", err)
	}
}

func TestVerifyTokenRejectsTamperedPayload(t *testing.T) {
	key := []byte("node-api-key")
	claims := Claims{SubscriptionID: "sub1", NodeID: "node1", Expiry: time.Now().Add(time.Hour).Unix()}
	token, err := IssueToken(key, claims)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	tampered := token[:len(token)-1] + "0"
	if _, err := VerifyToken(key, tampered, time.Now()); err == nil {
		t.Fatalf("VerifyToken() on tampered token expected error, got nil")
	}
}

func TestVerifyTokenRejectsMalformed(t *testing.T) {
	key := []byte("node-api-key")
	cases := []string{"", "no-dot-here", ".", "payload."}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := VerifyToken(key, tc, time.Now()); err == nil {
				t.Fatalf("VerifyToken(%q) expected error, got nil", tc)
			}
		})
	}
}
