package helps

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	utls "github.com/refraction-networking/utls"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

// OpenCode free-tier requests are only served to clients whose TLS
// ClientHello matches the one produced by the OpenCode CLI (a Bun/BoringSSL
// socket). Generic TLS stacks (Go, OpenSSL, Python) are answered with
// FreeUsageLimitError even when the HTTP request itself is byte-identical, so
// the free-tier path dials with that exact ClientHello via utls.
const (
	opencodeTLSUserAgent   = "opencode/latest/2.2.5/cli"
	opencodeTLSHost        = "opencode.ai"
	opencodeTLSDefaultPort = "443"

	// opencodeProjectFallbackID is a 40-hex project id in the shape the CLI
	// derives from its working directory. Upstream does not bind it to an
	// account; it only has to look like a CLI project hash.
	opencodeProjectFallbackID = "10c6333468356007fc526c20d914d8a3534df363"

	opencodeAcceptEncoding = "gzip, deflate, br, zstd"
	opencodeTLSHandshake   = 15 * time.Second
)

// OpencodeFreeTLSEnabled reports whether a request should use the
// OpenCode-compatible TLS transport. Keyed (paid) requests keep the regular
// Go transport; only the keyless free tier is fingerprint-gated upstream.
func OpencodeFreeTLSEnabled(compat *config.OpenAICompatibility, baseURL, apiKey string) bool {
	if strings.TrimSpace(apiKey) != "" {
		return false
	}
	if OpencodeIdentityEnabled(compat) {
		return true
	}
	return OpencodeTLSUpstream(baseURL)
}

// OpencodeTLSUpstream reports whether a base URL points straight at the
// OpenCode upstream rather than at a relay or a third-party gateway.
func OpencodeTLSUpstream(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "https") && isOpencodeUpstreamHost(parsed.Hostname())
}

// NewOpencodeHTTPClient builds an HTTP client whose TLS handshake matches the
// OpenCode CLI. When a proxy is configured for the request the regular
// proxy-aware client is returned: proxying would replace the handshake with
// the proxy's own, so the fingerprint cannot be carried through it.
func NewOpencodeHTTPClient(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth, timeout time.Duration) *http.Client {
	if effectiveProxyURL(ctx, cfg, auth) != "" {
		return NewProxyAwareHTTPClient(ctx, cfg, auth, timeout)
	}
	client := &http.Client{
		Transport: &opencodeTLSRoundTripper{
			dialer: &net.Dialer{Timeout: opencodeTLSHandshake, KeepAlive: 30 * time.Second},
			base:   http.DefaultTransport,
		},
	}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}

// opencodeClientHelloSpec reproduces the ClientHello of the OpenCode CLI's
// Bun socket client: 17 cipher suites, X25519/P-256/P-384 groups, no ALPN,
// no GREASE and no padding extension.
func opencodeClientHelloSpec() *utls.ClientHelloSpec {
	return &utls.ClientHelloSpec{
		TLSVersMin: tls.VersionTLS12,
		TLSVersMax: tls.VersionTLS13,
		CipherSuites: []uint16{
			4865, 4866, 4867,
			49195, 49199, 49196, 49200,
			52393, 52392,
			49161, 49171, 49162, 49172,
			156, 157, 47, 53,
		},
		CompressionMethods: []uint8{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.ExtendedMasterSecretExtension{},
			&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256, utls.CurveP384}},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
			&utls.SessionTicketExtension{},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				1027, 2052, 1025, 1283, 2053, 1281, 2054, 1537, 513,
			}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.SupportedVersionsExtension{Versions: []uint16{tls.VersionTLS13, tls.VersionTLS12}},
		},
	}
}

type opencodeTLSRoundTripper struct {
	dialer *net.Dialer
	base   http.RoundTripper
}

func (t *opencodeTLSRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil || !isOpencodeUpstreamHost(req.URL.Hostname()) || !strings.EqualFold(req.URL.Scheme, "https") {
		return t.fallback().RoundTrip(req)
	}

	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = opencodeTLSDefaultPort
	}
	conn, err := t.dialer.DialContext(req.Context(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("opencode tls: dial %s: %w", net.JoinHostPort(host, port), err)
	}

	uconn := utls.UClient(conn, &utls.Config{ServerName: host, MinVersion: tls.VersionTLS12}, utls.HelloCustom)
	if err := uconn.ApplyPreset(opencodeClientHelloSpec()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("opencode tls: apply client hello: %w", err)
	}
	hsCtx, cancel := context.WithTimeout(req.Context(), opencodeTLSHandshake)
	defer cancel()
	if err := uconn.HandshakeContext(hsCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("opencode tls: handshake: %w", err)
	}

	block, chunked := opencodeRequestHeaderBlock(req)
	bw := bufio.NewWriterSize(uconn, 32*1024)
	if _, err := bw.Write(block); err != nil {
		_ = uconn.Close()
		return nil, fmt.Errorf("opencode tls: write headers: %w", err)
	}
	if req.Body != nil {
		if chunked {
			cw := httputil.NewChunkedWriter(bw)
			_, err = io.Copy(cw, req.Body)
			if cerr := cw.Close(); err == nil {
				err = cerr
			}
		} else {
			_, err = io.Copy(bw, req.Body)
		}
		if err != nil {
			_ = uconn.Close()
			return nil, fmt.Errorf("opencode tls: write body: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		_ = uconn.Close()
		return nil, fmt.Errorf("opencode tls: flush: %w", err)
	}

	br := bufio.NewReaderSize(uconn, 32*1024)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = uconn.Close()
		return nil, fmt.Errorf("opencode tls: read response: %w", err)
	}
	body, err := opencodeResponseBody(resp, uconn)
	if err != nil {
		_ = resp.Body.Close()
		_ = uconn.Close()
		return nil, err
	}
	resp.Body = body
	return resp, nil
}

func (t *opencodeTLSRoundTripper) fallback() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

func isOpencodeUpstreamHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == opencodeTLSHost || strings.HasSuffix(host, "."+opencodeTLSHost)
}

// opencodeRequestHeaderBlock renders the request line and headers in the exact
// order the OpenCode CLI uses. The block includes the identity headers the
// free tier expects and is terminated by the blank line.
func opencodeRequestHeaderBlock(req *http.Request) ([]byte, bool) {
	session := req.Header.Get("x-opencode-session-id")
	if session == "" {
		session = req.Header.Get("x-opencode-session")
	}
	if session == "" {
		session = MintOpencodeSessionID()
	}
	project := strings.TrimSpace(req.Header.Get("x-opencode-project"))
	if len(project) != 40 || !isHexString(project) {
		project = opencodeProjectFallbackID
	}
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	traceID := randomHex(16)
	spanID := randomHex(8)

	path := req.URL.RequestURI()
	if path == "" {
		path = "/"
	}

	var b strings.Builder
	b.Grow(1024)
	fmt.Fprintf(&b, "%s %s HTTP/1.1\r\n", req.Method, path)
	b.WriteString("Authorization: Bearer public\r\n")
	fmt.Fprintf(&b, "Content-Type: %s\r\n", contentType)
	fmt.Fprintf(&b, "User-Agent: %s\r\n", opencodeTLSUserAgent)
	fmt.Fprintf(&b, "b3: %s-%s-1-%s\r\n", traceID, spanID, spanID)
	fmt.Fprintf(&b, "traceparent: 00-%s-%s-01\r\n", traceID, spanID)
	b.WriteString("x-opencode-client: cli\r\n")
	fmt.Fprintf(&b, "x-opencode-project: %s\r\n", project)
	fmt.Fprintf(&b, "x-opencode-session: %s\r\n", session)
	fmt.Fprintf(&b, "x-opencode-session-id: %s\r\n", session)
	fmt.Fprintf(&b, "x-session-affinity: %s\r\n", session)
	fmt.Fprintf(&b, "x-session-id: %s\r\n", session)

	for name, values := range req.Header {
		if opencodeCanonicalHeader(name) {
			continue
		}
		for _, value := range values {
			fmt.Fprintf(&b, "%s: %s\r\n", name, value)
		}
	}

	b.WriteString("Connection: keep-alive\r\n")
	b.WriteString("Accept: */*\r\n")
	fmt.Fprintf(&b, "Host: %s\r\n", req.URL.Host)
	fmt.Fprintf(&b, "Accept-Encoding: %s\r\n", opencodeAcceptEncoding)

	chunked := req.ContentLength < 0
	if chunked {
		b.WriteString("Transfer-Encoding: chunked\r\n")
	} else {
		fmt.Fprintf(&b, "Content-Length: %d\r\n", req.ContentLength)
	}
	b.WriteString("\r\n")
	return []byte(b.String()), chunked
}

func opencodeCanonicalHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "content-type", "user-agent", "b3", "traceparent",
		"x-opencode-client", "x-opencode-project", "x-opencode-session",
		"x-opencode-session-id", "x-session-affinity", "x-session-id",
		"connection", "accept", "host", "accept-encoding", "content-length",
		"transfer-encoding", "x-opencode-request":
		return true
	default:
		return false
	}
}

// opencodeResponseBody wraps the response body so that closing it also closes
// the TLS connection, and transparently decodes the encodings advertised by
// the request headers.
func opencodeResponseBody(resp *http.Response, conn net.Conn) (io.ReadCloser, error) {
	body := resp.Body
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding != "" && encoding != "identity" {
		var reader io.Reader
		var decoder io.Closer
		switch encoding {
		case "gzip", "x-gzip":
			zr, err := gzip.NewReader(body)
			if err != nil {
				return nil, fmt.Errorf("opencode tls: gzip reader: %w", err)
			}
			reader = zr
			decoder = zr
		case "deflate":
			zr, err := zlib.NewReader(body)
			if err != nil {
				fr := flate.NewReader(body)
				reader = fr
				decoder = fr
			} else {
				reader = zr
				decoder = zr
			}
		case "br":
			reader = brotli.NewReader(body)
		case "zstd":
			zr, err := zstd.NewReader(body)
			if err != nil {
				return nil, fmt.Errorf("opencode tls: zstd reader: %w", err)
			}
			reader = zr
			decoder = closerFunc(func() { zr.Close() })
		default:
			return nil, fmt.Errorf("opencode tls: unsupported content-encoding %q", encoding)
		}
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
		resp.Uncompressed = true
		return &opencodeCloser{Reader: reader, closers: []io.Closer{decoder, body, conn}}, nil
	}
	return &opencodeCloser{Reader: body, closers: []io.Closer{body, conn}}, nil
}

type closerFunc func()

func (f closerFunc) Close() error {
	f()
	return nil
}

type opencodeCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *opencodeCloser) Close() error {
	var first error
	for _, closer := range c.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := crand.Read(buf); err != nil {
		// crypto/rand failure is fatal for identity freshness; fall back to a
		// fixed value rather than emitting a broken header.
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(buf)
}

func isHexString(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return value != ""
}
