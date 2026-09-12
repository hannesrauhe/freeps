//go:build !notuya

package tuya

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const tcpPort = 6668

// errReadTimeout signals "no complete frame within the read deadline".
// Callers can distinguish it from real connection errors and keep reading.
var errReadTimeout = errors.New("tuya: read timeout")

// Device is a handle for one Tuya WiFi device on the local network.
type Device struct {
	ID      string  // device id (18 or 22 chars)
	Key     string  // local key, exactly 16 chars (from the Tuya cloud)
	IP      string  // IP address; must be reachable on the LAN
	Version float64 // protocol version: 3.1, 3.2, 3.3, 3.4 or 3.5

	dialTimeout time.Duration
	readTimeout time.Duration
}

func (d *Device) timeouts() (time.Duration, time.Duration) {
	dt, rt := d.dialTimeout, d.readTimeout
	if dt == 0 {
		dt = 5 * time.Second
	}
	if rt == 0 {
		rt = 5 * time.Second
	}
	return dt, rt
}

func (d *Device) keyBytes() ([]byte, error) {
	k := []byte(d.Key)
	if d.Version > 3.1 && len(k) != 16 {
		return nil, fmt.Errorf("tuya: local key for version %.1f must be 16 chars, got %d", d.Version, len(k))
	}
	return k, nil
}

// Connect opens the TCP connection and, for protocol >= 3.4, negotiates the
// session key. The returned sessionKey is what all later frames are
// encrypted/signed with.
func (d *Device) Connect() (net.Conn, []byte, error) {
	key, err := d.keyBytes()
	if err != nil {
		return nil, nil, err
	}
	dt, rt := d.timeouts()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(d.IP, fmt.Sprint(tcpPort)), dt)
	if err != nil {
		return nil, nil, fmt.Errorf("tuya: connect %s: %w", d.IP, err)
	}
	if d.Version < 3.4 {
		return conn, key, nil
	}
	r := &frameReader{conn: conn}
	conn.SetReadDeadline(time.Now().Add(rt))
	sessionKey, err := d.negotiate(conn, r, key, rt)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, sessionKey, nil
}

// dpQueryPayload is the JSON sent for a DP_QUERY (get data points) request.
// Field order matters little, but all four fields must be present.
type dpQueryPayload struct {
	GwID  string `json:"gwId"`
	DevID string `json:"devId"`
	UID   string `json:"uid"`
	T     string `json:"t"`
}

func (d *Device) queryPayload() []byte {
	b, _ := json.Marshal(dpQueryPayload{
		GwID: d.ID, DevID: d.ID, UID: d.ID,
		T: fmt.Sprintf("%d", time.Now().Unix()),
	})
	return b
}

// SendQuery asks the device (on an open connection) to report all data points.
func (d *Device) SendQuery(conn net.Conn, sessionKey []byte, seqno uint32) error {
	req, err := d.encode(seqno, cmdDPQuery, d.queryPayload(), sessionKey)
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(d.writeTimeout()))
	_, err = conn.Write(req)
	return err
}

// SendHeartbeat sends a keep-alive frame on an open connection.
func (d *Device) SendHeartbeat(conn net.Conn, sessionKey []byte, seqno uint32) error {
	payload := fmt.Sprintf(`{"gwId":"%s","devId":"%s"}`, d.ID, d.ID)
	req, err := d.encode(seqno, cmdHeartbeat, []byte(payload), sessionKey)
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(d.writeTimeout()))
	_, err = conn.Write(req)
	return err
}

func (d *Device) writeTimeout() time.Duration {
	if d.readTimeout > 0 {
		return d.readTimeout
	}
	return 5 * time.Second
}

// Status performs a one-shot query: connect, negotiate, query, read one
// answer, close. Prefer the persistent connection in normal operation —
// a Tuya device only accepts ONE TCP connection at a time.
func (d *Device) Status() (map[string]interface{}, error) {
	conn, sessionKey, err := d.Connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_, rt := d.timeouts()

	if err := d.SendQuery(conn, sessionKey, 1); err != nil {
		return nil, fmt.Errorf("tuya: send query: %w", err)
	}
	r := &frameReader{conn: conn}
	var lastErr error
	for i := 0; i < 3; i++ {
		conn.SetReadDeadline(time.Now().Add(rt))
		msg, err := r.receive(sessionKey, d.Version)
		if err != nil {
			lastErr = err
			continue
		}
		if len(msg.payload) == 0 {
			lastErr = fmt.Errorf("tuya: empty response frame (cmd %d)", msg.cmd)
			continue
		}
		return d.decodePayload(msg.payload, sessionKey)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tuya: no response from %s", d.IP)
	}
	return nil, lastErr
}

// negotiate performs the 3.4/3.5 session key exchange right after connect.
func (d *Device) negotiate(conn net.Conn, r *frameReader, realKey []byte, rt time.Duration) ([]byte, error) {
	localNonce := randomBytes(16)

	// step 1: send our nonce
	req, err := d.encode(1, cmdSessKeyNegStart, localNonce, realKey)
	if err != nil {
		return nil, err
	}
	conn.SetWriteDeadline(time.Now().Add(rt))
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("tuya: session step 1: %w", err)
	}

	// step 2: device answers with its nonce + HMAC
	conn.SetReadDeadline(time.Now().Add(rt))
	resp, err := r.receive(realKey, d.Version)
	if err != nil {
		return nil, fmt.Errorf("tuya: session step 2: %w", err)
	}
	if resp.cmd != cmdSessKeyNegResp {
		return nil, fmt.Errorf("tuya: session step 2: unexpected cmd %d", resp.cmd)
	}
	body := resp.payload
	if d.Version == 3.4 {
		body, err = aesECBDecrypt(realKey, body)
		if err != nil {
			return nil, fmt.Errorf("tuya: session step 2 decrypt: %w", err)
		}
	}
	if len(body) < 48 {
		return nil, fmt.Errorf("tuya: session step 2: payload too short (%d)", len(body))
	}
	remoteNonce := body[:16]
	if !hmacEqual(hmacSHA256(realKey, localNonce), body[16:48]) {
		return nil, fmt.Errorf("tuya: session step 2: HMAC mismatch (wrong local key?)")
	}

	// step 3: prove we know the key over the remote nonce
	finish, err := d.encode(2, cmdSessKeyNegFinish, hmacSHA256(realKey, remoteNonce), realKey)
	if err != nil {
		return nil, err
	}
	conn.SetWriteDeadline(time.Now().Add(rt))
	if _, err := conn.Write(finish); err != nil {
		return nil, fmt.Errorf("tuya: session step 3: %w", err)
	}

	// derive session key: AES(realKey, localNonce XOR remoteNonce)
	xor := make([]byte, 16)
	for i := range xor {
		xor[i] = localNonce[i] ^ remoteNonce[i]
	}
	if d.Version == 3.4 {
		return aesECBEncryptNoPad(realKey, xor)
	}
	// 3.5: AES-GCM with iv = localNonce[:12], session key = ciphertext[0:16]
	gcm, err := newGCM(realKey)
	if err != nil {
		return nil, err
	}
	ct, err := gcmEncrypt(gcm, localNonce[:12], xor, nil)
	if err != nil {
		return nil, err
	}
	if len(ct) < 16 {
		return nil, fmt.Errorf("tuya: session key derivation too short")
	}
	return ct[:16], nil
}

func (d *Device) encode(seqno, cmd uint32, payload, key []byte) ([]byte, error) {
	switch {
	case d.Version >= 3.5:
		if !noHeaderCmds[cmd] {
			payload = append(versionHeader(d.Version), payload...)
		}
		iv := randomBytes(12)
		return pack6699(seqno, cmd, payload, key, iv)
	case d.Version == 3.4:
		if !noHeaderCmds[cmd] {
			payload = append(versionHeader(d.Version), payload...)
		}
		enc, err := aesECBEncrypt(key, payload)
		if err != nil {
			return nil, err
		}
		return pack55AA(seqno, cmd, enc, key)
	case d.Version >= 3.2: // 3.2, 3.3
		enc, err := aesECBEncrypt(key, payload)
		if err != nil {
			return nil, err
		}
		if !noHeaderCmds[cmd] {
			enc = append(versionHeader(d.Version), enc...)
		}
		return pack55AA(seqno, cmd, enc, nil)
	default: // 3.1: unencrypted except CONTROL (not needed for read-only)
		return pack55AA(seqno, cmd, payload, nil)
	}
}

// decodePayload turns a response frame payload into the DPS map.
func (d *Device) decodePayload(payload, key []byte) (map[string]interface{}, error) {
	var raw []byte
	var err error
	switch {
	case d.Version >= 3.4: // payload already decrypted by unpack (3.5) or here (3.4)
		if d.Version == 3.4 {
			raw, err = aesECBDecrypt(key, payload)
			if err != nil {
				return nil, fmt.Errorf("tuya: decrypt response: %w", err)
			}
		} else {
			raw = payload
		}
		raw = stripVersionHeader(raw)
	case d.Version >= 3.2: // 3.2/3.3: header in clear, then AES-ECB
		raw = stripVersionHeader(payload)
		raw, err = aesECBDecrypt(key, raw)
		if err != nil {
			return nil, fmt.Errorf("tuya: decrypt response: %w", err)
		}
	default: // 3.1: "3.1" + 16-byte md5 digest + base64(AES-ECB)
		if len(payload) > 3 && string(payload[:3]) == "3.1" {
			b64 := payload[3+16:]
			enc, derr := base64.StdEncoding.DecodeString(string(b64))
			if derr != nil {
				return nil, fmt.Errorf("tuya: 3.1 response base64: %w", derr)
			}
			raw, err = aesECBDecrypt(key, enc)
			if err != nil {
				return nil, fmt.Errorf("tuya: 3.1 decrypt: %w", err)
			}
		} else {
			raw = payload
		}
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("tuya: response is not JSON: %w (%.80q)", err, raw)
	}
	// 3.3/3.4 devices nest the dps under "data"; 3.1/3.5 put them top-level
	dpsAny, ok := parsed["dps"]
	if !ok {
		if data, ok2 := parsed["data"].(map[string]interface{}); ok2 {
			dpsAny, ok = data["dps"]
		}
	}
	if !ok {
		if s, isStr := parsed["dps"].(string); isStr {
			// some firmwares double-encode dps as a JSON string
			var inner map[string]interface{}
			if err := json.Unmarshal([]byte(s), &inner); err == nil {
				return inner, nil
			}
		}
		return nil, fmt.Errorf("tuya: no dps in response: %.120q", raw)
	}
	switch dps := dpsAny.(type) {
	case map[string]interface{}:
		return dps, nil
	case string:
		var inner map[string]interface{}
		if err := json.Unmarshal([]byte(dps), &inner); err != nil {
			return nil, fmt.Errorf("tuya: dps string not JSON: %w", err)
		}
		return inner, nil
	}
	return nil, fmt.Errorf("tuya: unexpected dps type %T", dpsAny)
}

func stripVersionHeader(b []byte) []byte {
	s := string(b)
	for _, v := range []string{"3.1", "3.2", "3.3", "3.4", "3.5"} {
		if strings.HasPrefix(s, v) {
			// version bytes + 12 NULs
			if len(b) >= 15 && b[3] == 0 {
				return b[15:]
			}
			if len(b) >= 3 {
				return b[3:]
			}
		}
	}
	return b
}

// frameReader reads complete tuya frames from a TCP connection.
type frameReader struct {
	conn net.Conn
	buf  []byte
}

const minFrameLen = 28

func (r *frameReader) fill(n int) error {
	for len(r.buf) < n {
		tmp := make([]byte, 4096)
		n2, err := r.conn.Read(tmp)
		if n2 > 0 {
			r.buf = append(r.buf, tmp[:n2]...)
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return errReadTimeout
			}
			if err == io.EOF {
				return fmt.Errorf("tuya: connection closed by device")
			}
			return err
		}
	}
	return nil
}

// receive reads one frame and decodes it.
func (r *frameReader) receive(key []byte, version float64) (*message55AA, error) {
	if err := r.fill(minFrameLen); err != nil {
		return nil, err
	}
	// resynchronise onto a frame prefix
	for {
		_, _, _, _, err := parseHeader(r.buf)
		if err == nil {
			break
		}
		if len(r.buf) > 3 {
			r.buf = r.buf[3:]
		} else {
			r.buf = r.buf[len(r.buf):]
		}
		if err := r.fill(minFrameLen); err != nil {
			return nil, err
		}
	}
	_, _, _, total, err := parseHeader(r.buf)
	if err != nil {
		return nil, err
	}
	if err := r.fill(total); err != nil {
		return nil, err
	}
	frame := r.buf[:total]
	r.buf = r.buf[total:]

	prefix := binary.BigEndian.Uint32(frame[0:])
	if prefix == prefix6699 {
		return unpack6699(frame, key)
	}
	var hmacKey []byte
	if version >= 3.4 {
		hmacKey = key
	}
	return unpack55AA(frame, hmacKey)
}
