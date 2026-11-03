package dj

// A small websocket client, hand-rolled for one purpose: the speech
// service's socket has to go through the system's proxy, which the
// websocket libraries here do not do. It speaks just what the service
// needs — text frames out, text and binary frames in — and nothing more.

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// wsConn is a websocket session over TLS.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader

	wmu sync.Mutex // one write at a time
}

// wsDial opens the socket at a wss URL, through the system's proxy when
// the environment names one.
func wsDial(rawurl string, headers [][2]string) (*wsConn, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "443")
	}
	var conn net.Conn
	if proxy := os.Getenv("HTTPS_PROXY"); proxy != "" {
		if conn, err = wsThroughProxy(proxy, host); err != nil {
			return nil, err
		}
	} else {
		if conn, err = wsPlainDial(host); err != nil {
			return nil, err
		}
	}
	conn = tls.Client(conn, &tls.Config{ServerName: strings.Split(u.Host, ":")[0]})

	// The upgrade: a GET with the websocket's own headers.
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		conn.Close()
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "GET %s HTTP/1.1\r\nHost: %s\r\n", u.RequestURI(), u.Host)
	fmt.Fprintf(&b, "Upgrade: websocket\r\nConnection: Upgrade\r\n")
	fmt.Fprintf(&b, "Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n", base64.StdEncoding.EncodeToString(key))
	for _, h := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", h[0], h[1])
	}
	b.WriteString("\r\n")
	conn.SetDeadline(time.Now().Add(edgeDial))
	if _, err := io.WriteString(conn, b.String()); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(line, "101") {
		conn.Close()
		return nil, fmt.Errorf("dj: the voice service refused the handshake: %s", strings.TrimSpace(line))
	}
	// The rest of the response's head, through its blank line.
	for {
		l, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if strings.TrimSpace(l) == "" {
			break
		}
	}
	return &wsConn{conn: conn, br: br}, nil
}

// wsThroughProxy reaches host by asking the proxy for a tunnel.
func wsThroughProxy(proxy, host string) (net.Conn, error) {
	p := proxy
	if !strings.Contains(p, "://") {
		p = "http://" + p
	}
	pu, err := url.Parse(p)
	if err != nil {
		return nil, err
	}
	phost := pu.Host
	if _, _, err := net.SplitHostPort(phost); err != nil {
		phost = net.JoinHostPort(phost, "80")
	}
	conn, err := net.DialTimeout("tcp", phost, edgeDial)
	if err != nil {
		return nil, err
	}
	req := "CONNECT " + host + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n"
	conn.SetDeadline(time.Now().Add(edgeDial))
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	rd := bufio.NewReader(conn)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if strings.HasPrefix(line, "HTTP/") && !strings.Contains(line, " 200") {
			conn.Close()
			return nil, fmt.Errorf("dj: the proxy refused the tunnel: %s", strings.TrimSpace(line))
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	conn.SetDeadline(time.Time{})
	return conn, nil
}

func wsPlainDial(host string) (net.Conn, error) {
	return net.DialTimeout("tcp", host, edgeDial)
}

// SendText sends one text frame, whole and masked.
func (c *wsConn) SendText(s string) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	payload := []byte(s)
	var head [14]byte
	head[0] = 0x81 // FIN, text
	n := len(payload)
	maskBit := byte(0x80)
	switch {
	case n < 126:
		head[1] = maskBit | byte(n)
		copy(head[2:6], wsMask())
		_, err := c.conn.Write(append(head[:6], payload...))
		return err
	case n <= 0xFFFF:
		head[1] = maskBit | 126
		binary.BigEndian.PutUint16(head[2:4], uint16(n))
		copy(head[4:8], wsMask())
		_, err := c.conn.Write(head[:8])
		if err != nil {
			return err
		}
		return wsWriteMasked(c.conn, payload, head[4:8])
	default:
		head[1] = maskBit | 127
		binary.BigEndian.PutUint64(head[2:10], uint64(n))
		copy(head[10:14], wsMask())
		_, err := c.conn.Write(head[:14])
		if err != nil {
			return err
		}
		return wsWriteMasked(c.conn, payload, head[10:14])
	}
}

func wsMask() []byte {
	m := make([]byte, 4)
	rand.Read(m)
	return m
}

func wsWriteMasked(w io.Writer, payload, mask []byte) error {
	out := make([]byte, len(payload))
	for i, x := range payload {
		out[i] = x ^ mask[i%4]
	}
	_, err := w.Write(out)
	return err
}

// Receive waits for the next complete message, joining fragments, and
// answers pings on the way. The second result is true for binary frames.
func (c *wsConn) Receive() ([]byte, bool, error) {
	var msg []byte
	op := byte(0)
	for {
		var head [2]byte
		if _, err := io.ReadFull(c.br, head[:]); err != nil {
			return nil, false, err
		}
		fin := head[0]&0x80 != 0
		opcode := head[0] & 0x0F
		masked := head[1]&0x80 != 0
		n := int64(head[1] & 0x7F)
		switch n {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(c.br, ext[:]); err != nil {
				return nil, false, err
			}
			n = int64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(c.br, ext[:]); err != nil {
				return nil, false, err
			}
			n = int64(binary.BigEndian.Uint64(ext[:]))
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(c.br, mask[:]); err != nil {
				return nil, false, err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(c.br, payload); err != nil {
			return nil, false, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case 8: // close
			return nil, false, io.EOF
		case 9: // ping: answer with a pong
			if err := c.pong(payload); err != nil {
				return nil, false, err
			}
			continue
		case 10: // pong
			continue
		}
		if opcode <= 2 {
			op = opcode
			msg = append(msg, payload...)
		}
		if fin {
			return msg, op == 2, nil
		}
	}
}

func (c *wsConn) pong(payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	head := []byte{0x8A, 0x80, 0, 0, 0, 0}
	_, err := c.conn.Write(append(head, payload...))
	return err
}

func (c *wsConn) Close() { c.conn.Close() }
