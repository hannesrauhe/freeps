//go:build !notuya

// Package tuya reads (and controls) Tuya WiFi devices over the local LAN,
// without the Tuya cloud. The protocol (framing, AES, session key
// negotiation) is implemented natively; protocol versions 3.1, 3.2, 3.3,
// 3.4 and 3.5 are supported.
//
// Device IDs and local keys are obtained once via the Tuya cloud (see
// tools/tuya/probe.py); after that, no cloud access is needed.
package tuya

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Tuya command codes (from tuya-iotos-embeded-sdk lan_protocol.h)
const (
	cmdSessKeyNegStart  = 3
	cmdSessKeyNegResp   = 4
	cmdSessKeyNegFinish = 5
	cmdControl          = 7
	cmdHeartbeat        = 9
	cmdDPQuery          = 0x0a
	cmdUpdatedps        = 0x12
)

// Frame prefixes / suffixes
const (
	prefix55AA uint32 = 0x000055AA
	suffix55AA uint32 = 0x0000AA55
	prefix6699        = 0x6699
	suffix6699 uint32 = 0x00009966
)

const maxPayloadLength = 1440

// versionHeader is prepended (in clear or encrypted, depending on version)
// to payloads of commands that are not in noHeaderCmds.
func versionHeader(version float64) []byte {
	return append([]byte(fmt.Sprintf("%.1f", version)), make([]byte, 12)...)
}

// noHeaderCmds are commands that are sent without the "3.x\x00..." protocol
// header (and whose responses may or may not carry one).
var noHeaderCmds = map[uint32]bool{
	cmdDPQuery: true, cmdUpdatedps: true, cmdHeartbeat: true,
	cmdSessKeyNegStart: true, cmdSessKeyNegResp: true, cmdSessKeyNegFinish: true,
	0x0d: true, 0x40: true,
}

var errShortFrame = errors.New("tuya: frame too short / truncated")

// pack55AA builds a classic 0x55AA frame:
//
//	prefix | seqno | cmd | length | payload | crc | suffix
//
// length counts payload+crc+suffix. If hmacKey is non-nil the crc field is a
// 32-byte HMAC-SHA256 (protocol >= 3.4), otherwise a 4-byte CRC32 (IEEE).
func pack55AA(seqno, cmd uint32, payload []byte, hmacKey []byte) []byte {
	endLen := 8
	if hmacKey != nil {
		endLen = 36
	}
	buf := make([]byte, 0, 16+len(payload)+endLen)
	hdr := make([]byte, 16)
	binary.BigEndian.PutUint32(hdr[0:], prefix55AA)
	binary.BigEndian.PutUint32(hdr[4:], seqno)
	binary.BigEndian.PutUint32(hdr[8:], cmd)
	binary.BigEndian.PutUint32(hdr[12:], uint32(len(payload)+endLen))
	buf = append(buf, hdr...)
	buf = append(buf, payload...)
	if hmacKey != nil {
		buf = append(buf, hmacSHA256(hmacKey, buf)...)
	} else {
		crc := make([]byte, 4)
		binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(buf))
		buf = append(buf, crc...)
	}
	sfx := make([]byte, 4)
	binary.BigEndian.PutUint32(sfx, suffix55AA)
	return append(buf, sfx...)
}

// message55AA is a decoded 0x55AA frame.
type message55AA struct {
	seqno    uint32
	cmd      uint32
	retcode  uint32
	payload  []byte
	crcGood  bool
	prefixOK bool // false for 6699 frames
}

// parseHeader55AA parses a frame header (works for both 55AA and 6699).
// It returns the prefix, seqno, cmd and the total frame length.
func parseHeader(data []byte) (prefix uint32, seqno, cmd uint32, total int, err error) {
	if len(data) < 16 {
		return 0, 0, 0, 0, errShortFrame
	}
	p := binary.BigEndian.Uint32(data[0:])
	if p == prefix55AA {
		seqno = binary.BigEndian.Uint32(data[4:])
		cmd = binary.BigEndian.Uint32(data[8:])
		length := binary.BigEndian.Uint32(data[12:])
		if length > maxPayloadLength {
			return 0, 0, 0, 0, fmt.Errorf("tuya: corrupt frame, claims length %d", length)
		}
		return p, seqno, cmd, 16 + int(length), nil
	}
	if p == prefix6699 {
		seqno = binary.BigEndian.Uint32(data[6:])
		cmd = binary.BigEndian.Uint32(data[10:])
		length := binary.BigEndian.Uint32(data[14:])
		if length > maxPayloadLength {
			return 0, 0, 0, 0, fmt.Errorf("tuya: corrupt frame, claims length %d", length)
		}
		return p, seqno, cmd, 18 + int(length) + 4, nil
	}
	return 0, 0, 0, 0, fmt.Errorf("tuya: unknown frame prefix %08x", p)
}

// unpack55AA decodes a complete 0x55AA frame. hmacKey must match what the
// sender used (nil for CRC32 frames). A CRC/HMAC mismatch is reported via
// crcGood but not treated as an error (mirrors tinytuya behaviour).
func unpack55AA(data []byte, hmacKey []byte) (*message55AA, error) {
	prefix, seqno, cmd, total, err := parseHeader(data)
	if err != nil {
		return nil, err
	}
	if prefix != prefix55AA {
		return &message55AA{seqno: seqno, cmd: cmd, prefixOK: false}, nil
	}
	if len(data) < total {
		return nil, errShortFrame
	}
	endLen := 8
	if hmacKey != nil {
		endLen = 36
	}
	length := total - 16
	if length < 4+endLen {
		return nil, errShortFrame
	}
	retcode := binary.BigEndian.Uint32(data[16:])
	payload := data[20 : total-endLen]
	m := &message55AA{seqno: seqno, cmd: cmd, retcode: retcode, payload: payload, prefixOK: true}
	suffix := binary.BigEndian.Uint32(data[total-4:])
	if suffix != suffix55AA {
		return m, fmt.Errorf("tuya: wrong suffix %08x", suffix)
	}
	if hmacKey != nil {
		want := hmacSHA256(hmacKey, data[:total-endLen])
		m.crcGood = hmacEqual(want, data[total-endLen:total-4])
	} else {
		want := make([]byte, 4)
		binary.BigEndian.PutUint32(want, crc32.ChecksumIEEE(data[:total-endLen]))
		m.crcGood = string(want) == string(data[total-endLen:total-4])
	}
	return m, nil
}

// pack6699 builds a protocol 3.5 frame (AES-GCM):
//
//	prefix(4) | 0(2) | seqno(4) | cmd(4) | length(4) | iv(12) | ct+tag(16) | suffix(4)
//
// The GCM AAD is data[4:18] (everything between prefix and encrypted body).
func pack6699(seqno, cmd uint32, payload, key, iv []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	length := len(payload) + 12 + 16 // iv + tag (suffix NOT counted, as in tinytuya)
	hdr := make([]byte, 18)
	binary.BigEndian.PutUint32(hdr[0:], prefix6699)
	binary.BigEndian.PutUint16(hdr[4:], 0)
	binary.BigEndian.PutUint32(hdr[6:], seqno)
	binary.BigEndian.PutUint32(hdr[10:], cmd)
	binary.BigEndian.PutUint32(hdr[14:], uint32(length))
	ct, err := gcmEncrypt(gcm, iv, payload, hdr[4:18])
	if err != nil {
		return nil, err
	}
	buf := append([]byte{}, hdr...)
	buf = append(buf, iv...)
	buf = append(buf, ct...) // ct already includes the 16-byte tag
	sfx := make([]byte, 4)
	binary.BigEndian.PutUint32(sfx, suffix6699)
	return append(buf, sfx...), nil
}

// unpack6699 decodes a complete protocol 3.5 frame.
func unpack6699(data []byte, key []byte) (*message55AA, error) {
	prefix, seqno, cmd, total, err := parseHeader(data)
	if err != nil {
		return nil, err
	}
	if prefix != prefix6699 {
		return &message55AA{seqno: seqno, cmd: cmd, prefixOK: false}, nil
	}
	if len(data) < total {
		return nil, errShortFrame
	}
	body := data[18 : total-4] // iv | ct | tag
	if len(body) < 12+16 {
		return nil, errShortFrame
	}
	iv := body[:12]
	ct := body[12:]
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plain, err := gcmDecrypt(gcm, iv, ct, data[4:18])
	if err != nil {
		return nil, fmt.Errorf("tuya: 6699 frame failed authentication: %w", err)
	}
	m := &message55AA{seqno: seqno, cmd: cmd, payload: plain, crcGood: true, prefixOK: true}
	// retcode heuristic (as in tinytuya): if the plaintext does not start
	// with '{' but does after skipping 4 bytes, those 4 bytes are a retcode.
	if len(plain) > 5 && plain[0] != '{' && plain[4] == '{' {
		m.retcode = binary.BigEndian.Uint32(plain[:4])
		m.payload = plain[4:]
	}
	return m, nil
}
