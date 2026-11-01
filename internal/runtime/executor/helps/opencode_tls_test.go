package helps

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

// ja3OfClientHello computes the JA3 string/hash for a TLS handshake message.
func ja3OfClientHello(ch []byte) (string, string) {
	if len(ch) < 4 || ch[0] != 1 {
		return "", ""
	}
	p := 4 + 2 + 32
	if p >= len(ch) {
		return "", ""
	}
	sidLen := int(ch[p])
	p += 1 + sidLen
	if p+2 > len(ch) {
		return "", ""
	}
	csLen := int(ch[p])<<8 | int(ch[p+1])
	p += 2
	var ciphers []string
	for i := 0; i+1 < csLen && p+1 < len(ch); i += 2 {
		ciphers = append(ciphers, fmt.Sprintf("%d", int(ch[p])<<8|int(ch[p+1])))
		p += 2
	}
	if p >= len(ch) {
		return "", ""
	}
	compLen := int(ch[p])
	p += 1 + compLen
	if p+2 > len(ch) {
		return "", ""
	}
	extTotal := int(ch[p])<<8 | int(ch[p+1])
	p += 2
	end := p + extTotal
	var exts, groups, ecpf []string
	for p+4 <= end && p+4 <= len(ch) {
		et := int(ch[p])<<8 | int(ch[p+1])
		el := int(ch[p+2])<<8 | int(ch[p+3])
		data := ch[p+4 : p+4+el]
		p += 4 + el
		exts = append(exts, fmt.Sprintf("%d", et))
		switch et {
		case 10:
			if len(data) >= 2 {
				n := int(data[0])<<8 | int(data[1])
				for i := 2; i+1 < 2+n && i+1 < len(data); i += 2 {
					groups = append(groups, fmt.Sprintf("%d", int(data[i])<<8|int(data[i+1])))
				}
			}
		case 11:
			if len(data) >= 1 {
				n := int(data[0])
				for i := 1; i < 1+n && i < len(data); i++ {
					ecpf = append(ecpf, fmt.Sprintf("%d", data[i]))
				}
			}
		}
	}
	raw := strings.Join(ciphers, ",") + "," + strings.Join(exts, "-") + "," +
		strings.Join(groups, "-") + "," + strings.Join(ecpf, "-")
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:]), raw
}

func readClientHello(conn net.Conn) ([]byte, error) {
	var hs []byte
	for {
		hdr := make([]byte, 5)
		if _, err := ioReadFull(conn, hdr); err != nil {
			return nil, err
		}
		payload := make([]byte, int(hdr[3])<<8|int(hdr[4]))
		if _, err := ioReadFull(conn, payload); err != nil {
			return nil, err
		}
		if hdr[0] != 0x16 {
			return hs, nil
		}
		hs = append(hs, payload...)
		if len(hs) >= 4 {
			need := 4 + (int(hs[1]) << 16) + (int(hs[2]) << 8) + int(hs[3])
			if len(hs) >= need {
				return hs[:need], nil
			}
		}
	}
}

func ioReadFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// TestOpencodeClientHelloJA3 pins the ClientHello shape to the one the
// OpenCode CLI's Bun socket emits and opencode.ai accepts.
func TestOpencodeClientHelloJA3(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	helloCh := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		hello, err := readClientHello(conn)
		if err == nil {
			helloCh <- hello
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	uconn := utls.UClient(conn, &utls.Config{ServerName: opencodeTLSHost, MinVersion: tls.VersionTLS12}, utls.HelloCustom)
	if err := uconn.ApplyPreset(opencodeClientHelloSpec()); err != nil {
		t.Fatalf("apply preset: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = uconn.HandshakeContext(ctx) }()

	select {
	case hello := <-helloCh:
		got, raw := ja3OfClientHello(hello)
		const want = "eeb78b967c1db14e3c3ef9be3c242aa1"
		if got != want {
			t.Fatalf("JA3 mismatch:\n got %s\nwant %s\nraw %s", got, want, raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ClientHello")
	}
}

func TestOpencodeRequestHeaderBlockOrder(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.ContentLength = 0
	block, chunked := opencodeRequestHeaderBlock(req)
	if chunked {
		t.Fatal("expected fixed Content-Length, got chunked")
	}
	lines := strings.Split(string(block), "\r\n")
	if len(lines) < 2 || lines[len(lines)-2] != "" {
		t.Fatalf("header block not terminated by blank line: %q", string(block))
	}
	var names []string
	for _, line := range lines[:len(lines)-2] {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, http.MethodPost+" ") {
			names = append(names, "request-line")
			continue
		}
		names = append(names, strings.ToLower(strings.SplitN(line, ":", 2)[0]))
	}
	want := []string{
		"request-line",
		"authorization", "content-type", "user-agent", "b3", "traceparent",
		"x-opencode-client", "x-opencode-project", "x-opencode-session",
		"x-opencode-session-id", "x-session-affinity", "x-session-id",
		"connection", "accept", "host", "accept-encoding", "content-length",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("header order mismatch:\n got %v\nwant %v", names, want)
	}
	if !strings.Contains(string(block), "Authorization: Bearer public\r\n") {
		t.Fatal("missing public bearer authorization")
	}
	if !strings.Contains(string(block), "User-Agent: "+opencodeTLSUserAgent+"\r\n") {
		t.Fatal("missing CLI user agent")
	}
	if !strings.Contains(string(block), "b3: ") || !strings.Contains(string(block), "traceparent: 00-") {
		t.Fatal("missing trace headers")
	}
}

func TestIsOpencodeUpstreamHost(t *testing.T) {
	yes := []string{"opencode.ai", "OpenCode.AI", "models.opencode.ai"}
	no := []string{"opencode.ai.example.com", "notopencode.ai", "", "zenmux.ai"}
	for _, host := range yes {
		if !isOpencodeUpstreamHost(host) {
			t.Fatalf("expected %q to match", host)
		}
	}
	for _, host := range no {
		if isOpencodeUpstreamHost(host) {
			t.Fatalf("expected %q not to match", host)
		}
	}
}
