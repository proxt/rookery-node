package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriteReadDatagramRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello")},
		{"exactly-max", bytes.Repeat([]byte{0xAB}, MaxDatagramSize)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteDatagram(&buf, tc.payload); err != nil {
				t.Fatalf("WriteDatagram() error = %v", err)
			}

			got, err := ReadDatagram(&buf)
			if err != nil {
				t.Fatalf("ReadDatagram() error = %v", err)
			}
			if !bytes.Equal(got, tc.payload) {
				t.Fatalf("got %v, want %v", got, tc.payload)
			}
		})
	}
}

func TestWriteDatagramTooLarge(t *testing.T) {
	var buf bytes.Buffer
	payload := bytes.Repeat([]byte{0x01}, MaxDatagramSize+1)
	err := WriteDatagram(&buf, payload)
	if !errors.Is(err, ErrDatagramTooLarge) {
		t.Fatalf("WriteDatagram() error = %v, want ErrDatagramTooLarge", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("WriteDatagram() wrote %d bytes despite rejecting the payload", buf.Len())
	}
}

func TestMultipleDatagramsPreserveBoundaries(t *testing.T) {
	var buf bytes.Buffer
	payloads := [][]byte{[]byte("first"), []byte(""), []byte("third-payload")}

	for _, p := range payloads {
		if err := WriteDatagram(&buf, p); err != nil {
			t.Fatalf("WriteDatagram() error = %v", err)
		}
	}

	for i, want := range payloads {
		got, err := ReadDatagram(&buf)
		if err != nil {
			t.Fatalf("ReadDatagram() #%d error = %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("datagram #%d: got %v, want %v", i, got, want)
		}
	}
}

func TestReadDatagramGarbageInput(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated-length", []byte{0x00}},
		{"length-exceeds-data", []byte{0x00, 0x10, 'a', 'b'}},
		{"claims-max-but-empty", []byte{0xFF, 0xFF}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadDatagram(bytes.NewReader(tc.data))
			if err == nil {
				t.Fatalf("ReadDatagram(%x) expected error, got nil", tc.data)
			}
		})
	}
}
