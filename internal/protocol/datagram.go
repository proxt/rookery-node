package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxDatagramSize is the largest UDP payload a datagram frame can carry,
// matching the 2-byte length prefix.
const MaxDatagramSize = 65535

var (
	ErrDatagramTooLarge = errors.New("protocol: datagram too large")
)

// WriteDatagram writes payload to w as a length-prefixed frame: a UDP
// stream's smux stream is a byte pipe, so datagram boundaries must be framed
// explicitly to survive the trip.
func WriteDatagram(w io.Writer, payload []byte) error {
	if len(payload) > MaxDatagramSize {
		return ErrDatagramTooLarge
	}

	buf := make([]byte, 2+len(payload))
	binary.BigEndian.PutUint16(buf, uint16(len(payload)))
	copy(buf[2:], payload)

	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("protocol: write datagram: %w", err)
	}
	return nil
}

// ReadDatagram reads one length-prefixed frame from r.
func ReadDatagram(r io.Reader) ([]byte, error) {
	var lenBytes [2]byte
	if _, err := io.ReadFull(r, lenBytes[:]); err != nil {
		return nil, fmt.Errorf("protocol: read datagram length: %w", err)
	}

	n := binary.BigEndian.Uint16(lenBytes[:])
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("protocol: read datagram payload: %w", err)
	}
	return payload, nil
}
