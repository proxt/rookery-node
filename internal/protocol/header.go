// Package protocol defines the destination header exchanged as the first
// frame of every smux stream carried over the Rookery tunnel.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// AddrType identifies the encoding used for the destination address.
type AddrType byte

const (
	AddrTypeIPv4   AddrType = 0x01
	AddrTypeDomain AddrType = 0x03
	AddrTypeIPv6   AddrType = 0x04
)

// Proto identifies the transport protocol requested for the destination.
type Proto byte

const (
	ProtoTCP Proto = 0x01
	ProtoUDP Proto = 0x02
)

// MaxDomainLength is the largest domain name the wire format can carry, since
// the length prefix is a single byte.
const MaxDomainLength = 255

var (
	ErrUnknownAddrType = errors.New("protocol: unknown address type")
	ErrUnknownProto    = errors.New("protocol: unknown protocol")
	ErrDomainTooLong   = errors.New("protocol: domain name too long")
	ErrEmptyDomain     = errors.New("protocol: empty domain name")
	ErrInvalidAddr     = errors.New("protocol: invalid address for type")
)

// Header is the destination frame every smux stream starts with: which
// address to dial, on which port, over which transport protocol.
type Header struct {
	AddrType AddrType
	Addr     string
	Port     uint16
	Proto    Proto
}

// Encode serializes the header to its wire representation.
func (h Header) Encode() ([]byte, error) {
	if h.Proto != ProtoTCP && h.Proto != ProtoUDP {
		return nil, ErrUnknownProto
	}

	var addrBytes []byte
	switch h.AddrType {
	case AddrTypeIPv4:
		ip := net.ParseIP(h.Addr)
		v4 := ip.To4()
		if v4 == nil {
			return nil, fmt.Errorf("%w: %q is not a valid IPv4 address", ErrInvalidAddr, h.Addr)
		}
		addrBytes = v4
	case AddrTypeIPv6:
		ip := net.ParseIP(h.Addr)
		if ip == nil || ip.To4() != nil {
			return nil, fmt.Errorf("%w: %q is not a valid IPv6 address", ErrInvalidAddr, h.Addr)
		}
		addrBytes = ip.To16()
	case AddrTypeDomain:
		if len(h.Addr) == 0 {
			return nil, ErrEmptyDomain
		}
		if len(h.Addr) > MaxDomainLength {
			return nil, ErrDomainTooLong
		}
		addrBytes = append([]byte{byte(len(h.Addr))}, []byte(h.Addr)...)
	default:
		return nil, ErrUnknownAddrType
	}

	buf := make([]byte, 0, 1+len(addrBytes)+2+1)
	buf = append(buf, byte(h.AddrType))
	buf = append(buf, addrBytes...)
	buf = binary.BigEndian.AppendUint16(buf, h.Port)
	buf = append(buf, byte(h.Proto))
	return buf, nil
}

// WriteHeader encodes and writes the header to w.
func WriteHeader(w io.Writer, h Header) error {
	buf, err := h.Encode()
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	if err != nil {
		return fmt.Errorf("protocol: write header: %w", err)
	}
	return nil
}

// ReadHeader reads and decodes a header from r.
func ReadHeader(r io.Reader) (Header, error) {
	var typeByte [1]byte
	if _, err := io.ReadFull(r, typeByte[:]); err != nil {
		return Header{}, fmt.Errorf("protocol: read address type: %w", err)
	}

	addrType := AddrType(typeByte[0])
	var addr string
	switch addrType {
	case AddrTypeIPv4:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return Header{}, fmt.Errorf("protocol: read ipv4 address: %w", err)
		}
		addr = net.IP(b[:]).String()
	case AddrTypeIPv6:
		var b [16]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return Header{}, fmt.Errorf("protocol: read ipv6 address: %w", err)
		}
		addr = net.IP(b[:]).String()
	case AddrTypeDomain:
		var lenByte [1]byte
		if _, err := io.ReadFull(r, lenByte[:]); err != nil {
			return Header{}, fmt.Errorf("protocol: read domain length: %w", err)
		}
		n := int(lenByte[0])
		if n == 0 {
			return Header{}, ErrEmptyDomain
		}
		domain := make([]byte, n)
		if _, err := io.ReadFull(r, domain); err != nil {
			return Header{}, fmt.Errorf("protocol: read domain: %w", err)
		}
		addr = string(domain)
	default:
		return Header{}, fmt.Errorf("%w: 0x%02x", ErrUnknownAddrType, typeByte[0])
	}

	var portBytes [2]byte
	if _, err := io.ReadFull(r, portBytes[:]); err != nil {
		return Header{}, fmt.Errorf("protocol: read port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes[:])

	var protoByte [1]byte
	if _, err := io.ReadFull(r, protoByte[:]); err != nil {
		return Header{}, fmt.Errorf("protocol: read protocol: %w", err)
	}
	proto := Proto(protoByte[0])
	if proto != ProtoTCP && proto != ProtoUDP {
		return Header{}, fmt.Errorf("%w: 0x%02x", ErrUnknownProto, protoByte[0])
	}

	return Header{AddrType: addrType, Addr: addr, Port: port, Proto: proto}, nil
}
