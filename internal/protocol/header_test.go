package protocol

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		h    Header
	}{
		{"ipv4-tcp", Header{AddrType: AddrTypeIPv4, Addr: "192.168.1.1", Port: 443, Proto: ProtoTCP}},
		{"ipv4-udp", Header{AddrType: AddrTypeIPv4, Addr: "8.8.8.8", Port: 53, Proto: ProtoUDP}},
		{"ipv4-zero", Header{AddrType: AddrTypeIPv4, Addr: "0.0.0.0", Port: 0, Proto: ProtoTCP}},
		{"ipv6-tcp", Header{AddrType: AddrTypeIPv6, Addr: "2001:db8::1", Port: 8443, Proto: ProtoTCP}},
		{"ipv6-loopback", Header{AddrType: AddrTypeIPv6, Addr: "::1", Port: 80, Proto: ProtoTCP}},
		{"domain-short", Header{AddrType: AddrTypeDomain, Addr: "example.com", Port: 443, Proto: ProtoTCP}},
		{"domain-max-length", Header{AddrType: AddrTypeDomain, Addr: strings.Repeat("a", MaxDomainLength), Port: 443, Proto: ProtoTCP}},
		{"domain-single-char", Header{AddrType: AddrTypeDomain, Addr: "a", Port: 1, Proto: ProtoUDP}},
		{"port-max", Header{AddrType: AddrTypeIPv4, Addr: "10.0.0.1", Port: 65535, Proto: ProtoTCP}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := tc.h.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			got, err := ReadHeader(bytes.NewReader(buf))
			if err != nil {
				t.Fatalf("ReadHeader() error = %v", err)
			}

			if got != tc.h {
				t.Fatalf("round trip mismatch: got %+v, want %+v", got, tc.h)
			}
		})
	}
}

func TestWriteHeaderReadHeaderRoundTrip(t *testing.T) {
	h := Header{AddrType: AddrTypeDomain, Addr: "rookery.internal", Port: 22, Proto: ProtoTCP}

	var buf bytes.Buffer
	if err := WriteHeader(&buf, h); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}

	got, err := ReadHeader(&buf)
	if err != nil {
		t.Fatalf("ReadHeader() error = %v", err)
	}
	if got != h {
		t.Fatalf("got %+v, want %+v", got, h)
	}
}

func TestEncodeErrors(t *testing.T) {
	cases := []struct {
		name    string
		h       Header
		wantErr error
	}{
		{"unknown-addr-type", Header{AddrType: 0x99, Addr: "1.2.3.4", Port: 1, Proto: ProtoTCP}, ErrUnknownAddrType},
		{"unknown-proto", Header{AddrType: AddrTypeIPv4, Addr: "1.2.3.4", Port: 1, Proto: 0x99}, ErrUnknownProto},
		{"empty-domain", Header{AddrType: AddrTypeDomain, Addr: "", Port: 1, Proto: ProtoTCP}, ErrEmptyDomain},
		{"domain-too-long", Header{AddrType: AddrTypeDomain, Addr: strings.Repeat("a", MaxDomainLength+1), Port: 1, Proto: ProtoTCP}, ErrDomainTooLong},
		{"ipv4-invalid-string", Header{AddrType: AddrTypeIPv4, Addr: "not-an-ip", Port: 1, Proto: ProtoTCP}, ErrInvalidAddr},
		{"ipv4-given-ipv6", Header{AddrType: AddrTypeIPv4, Addr: "::1", Port: 1, Proto: ProtoTCP}, ErrInvalidAddr},
		{"ipv6-given-ipv4", Header{AddrType: AddrTypeIPv6, Addr: "1.2.3.4", Port: 1, Proto: ProtoTCP}, ErrInvalidAddr},
		{"ipv6-invalid-string", Header{AddrType: AddrTypeIPv6, Addr: "garbage", Port: 1, Proto: ProtoTCP}, ErrInvalidAddr},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.h.Encode()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Encode() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestReadHeaderGarbageInput(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"unknown-addr-type", []byte{0xFF, 0x00, 0x50, 0x01}},
		{"truncated-after-type", []byte{byte(AddrTypeIPv4)}},
		{"truncated-ipv4-address", []byte{byte(AddrTypeIPv4), 0x01, 0x02}},
		{"truncated-ipv6-address", []byte{byte(AddrTypeIPv6), 0x01, 0x02, 0x03}},
		{"truncated-domain-length", []byte{byte(AddrTypeDomain)}},
		{"domain-length-exceeds-data", []byte{byte(AddrTypeDomain), 0x10, 'a', 'b'}},
		{"zero-length-domain", []byte{byte(AddrTypeDomain), 0x00, 0x00, 0x50, 0x01}},
		{"missing-port", []byte{byte(AddrTypeIPv4), 1, 2, 3, 4}},
		{"missing-proto", []byte{byte(AddrTypeIPv4), 1, 2, 3, 4, 0x00, 0x50}},
		{"unknown-proto", []byte{byte(AddrTypeIPv4), 1, 2, 3, 4, 0x00, 0x50, 0xEE}},
		{"random-bytes", []byte{0x13, 0x37, 0xDE, 0xAD, 0xBE, 0xEF}},
		{"all-zeros", make([]byte, 20)},
		{"all-ones", bytes.Repeat([]byte{0xFF}, 20)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadHeader(bytes.NewReader(tc.data))
			if err == nil {
				t.Fatalf("ReadHeader(%x) expected error, got nil", tc.data)
			}
		})
	}
}

func TestReadHeaderDoesNotOverreadStream(t *testing.T) {
	h := Header{AddrType: AddrTypeIPv4, Addr: "1.2.3.4", Port: 80, Proto: ProtoTCP}
	buf, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	payload := []byte("trailing-stream-payload")
	full := append(buf, payload...)

	r := bytes.NewReader(full)
	got, err := ReadHeader(r)
	if err != nil {
		t.Fatalf("ReadHeader() error = %v", err)
	}
	if got != h {
		t.Fatalf("got %+v, want %+v", got, h)
	}

	rest := make([]byte, len(payload))
	if _, err := io.ReadFull(r, rest); err != nil {
		t.Fatalf("reading trailing payload: %v", err)
	}
	if !bytes.Equal(rest, payload) {
		t.Fatalf("trailing payload corrupted: got %q, want %q", rest, payload)
	}
}
