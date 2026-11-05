package rtr

import (
	"encoding/binary"
	"fmt"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"io"
)

const (
	Notify        = 0
	SerialQuery   = 1
	ResetQuery    = 2
	CacheResponse = 3
	IPv4          = 4
	IPv6          = 6
	EndOfData     = 7
	CacheReset    = 8
	ErrorReport   = 10
	MaxPDU        = 4096
)

var be = binary.BigEndian

type PDU struct {
	Version, Type uint8
	Field         uint16
	Body          []byte
	raw           []byte // Preserve malformed input verbatim for Error Report encapsulation.
}

func (p PDU) Bytes() []byte {
	b := make([]byte, 8+len(p.Body))
	b[0] = p.Version
	b[1] = p.Type
	be.PutUint16(b[2:], p.Field)
	be.PutUint32(b[4:], uint32(len(b)))
	copy(b[8:], p.Body)
	return b
}
func Read(r io.Reader) (PDU, error) {
	var h [8]byte
	_, err := io.ReadFull(r, h[:])
	if err != nil {
		return PDU{}, err
	}
	p := PDU{Version: h[0], Type: h[1], Field: be.Uint16(h[2:])}
	n := be.Uint32(h[4:])
	if n < 8 || n > MaxPDU {
		p.raw = append([]byte(nil), h[:]...)
		return p, fmt.Errorf("invalid PDU length %d", n)
	}
	p.Body = make([]byte, int(n)-8)
	read, err := io.ReadFull(r, p.Body)
	if err != nil {
		p.raw = append(append([]byte(nil), h[:]...), p.Body[:read]...)
		return p, err
	}
	return p, nil
}
func Write(w io.Writer, p PDU) error {
	b := p.Bytes()
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func New(t uint8, f uint16, values ...uint32) PDU {
	return NewVersion(1, t, f, values...)
}
func NewVersion(version, t uint8, f uint16, values ...uint32) PDU {
	p := PDU{Version: version, Type: t, Field: f, Body: make([]byte, 4*len(values))}
	for i, v := range values {
		be.PutUint32(p.Body[i*4:], v)
	}
	return p
}
func Prefix(r vrp.Record, announce bool) PDU {
	return PrefixVersion(1, r, announce)
}
func PrefixVersion(version uint8, r vrp.Record, announce bool) PDU {
	n, t := 4, uint8(IPv4)
	if r.Prefix.Addr().Is6() {
		n, t = 16, IPv6
	}
	p := PDU{Version: version, Type: t, Body: make([]byte, 8+n)}
	if announce {
		p.Body[0] = 1
	}
	p.Body[1] = byte(r.Prefix.Bits())
	p.Body[2] = r.MaxLen
	copy(p.Body[4:], r.Prefix.Addr().AsSlice())
	be.PutUint32(p.Body[4+n:], r.ASN)
	return p
}
func Error(code uint16, p PDU, message string) PDU {
	return ErrorVersion(1, code, p, message)
}
func ErrorVersion(version uint8, code uint16, p PDU, message string) PDU {
	raw := p.Bytes()
	if p.raw != nil {
		raw = p.raw
	}
	b := make([]byte, 8+len(raw)+len(message))
	be.PutUint32(b, uint32(len(raw)))
	copy(b[4:], raw)
	be.PutUint32(b[4+len(raw):], uint32(len(message)))
	copy(b[8+len(raw):], message)
	return PDU{Version: version, Type: ErrorReport, Field: code, Body: b}
}

// EndOfDataVersion encodes the protocol-version-specific End of Data PDU.
// RTR v0 (RFC 6810) carries only the serial number; RTR v1 (RFC 8210)
// additionally carries refresh, retry, and expire intervals.
func EndOfDataVersion(version uint8, session uint16, serial, refresh, retry, expire uint32) PDU {
	if version == 0 {
		return NewVersion(0, EndOfData, session, serial)
	}
	return NewVersion(version, EndOfData, session, serial, refresh, retry, expire)
}
