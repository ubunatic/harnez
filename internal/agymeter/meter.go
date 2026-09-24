// Package agymeter implements the opt-in, per-process Antigravity usage proxy.
package agymeter

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const host = "daily-cloudcode-pa.googleapis.com"

type Record struct {
	Time         time.Time `json:"time"`
	Kind         string    `json:"kind"`
	Session      string    `json:"session,omitempty"`
	Conversation string    `json:"conversation,omitempty"`
	PromptID     string    `json:"prompt_id,omitempty"`
	Model        string    `json:"model,omitempty"`
	Prompt       int64     `json:"promptTokenCount,omitempty"`
	Candidates   int64     `json:"candidatesTokenCount,omitempty"`
	Thoughts     int64     `json:"thoughtsTokenCount,omitempty"`
	Cached       int64     `json:"cachedContentTokenCount,omitempty"`
	Total        int64     `json:"totalTokenCount,omitempty"`
	Bucket       string    `json:"bucketId,omitempty"`
	Remaining    *float64  `json:"remainingFraction,omitempty"`
	Reset        string    `json:"resetTime,omitempty"`
}

type Meter struct {
	listener  net.Listener
	server    *http.Server
	cert      tls.Certificate
	logPath   string
	mu        sync.Mutex
	debug     bool
	leaves    sync.Map
	sessionID string
	promptID  string
	lastQuota map[string]Record
}

// Run starts a private meter for the lifetime of the child command. Failure to
// initialize metering is fail-open: the requested program still runs plainly.
func Run(ctx context.Context, home string, command string, args []string, stdout, stderr io.Writer) error {
	return RunWithEnv(ctx, home, command, args, os.Environ(), stdout, stderr)
}

// RunWithEnv starts a per-command meter using an explicit child environment.
func RunWithEnv(ctx context.Context, home, command string, args, environ []string, stdout, stderr io.Writer) error {
	return RunWithEnvDir(ctx, home, command, args, environ, "", stdout, stderr)
}

// RunWithEnvDir is RunWithEnv with an explicit child working directory.
func RunWithEnvDir(ctx context.Context, home, command string, args, environ []string, dir string, stdout, stderr io.Writer) error {
	env := append([]string(nil), environ...)
	sessionID := envValue(env, "HARNEZ_SESSION_ID")
	promptID, idErr := newPromptID()
	if idErr != nil {
		fmt.Fprintln(stderr, "harnez-agy: metering ID unavailable; starting agy without metering")
		return runChild(ctx, command, args, env, dir, stdout, stderr)
	}
	m, err := newMeter(home, sessionID, promptID)
	if err != nil {
		fmt.Fprintln(stderr, "harnez-agy: metering proxy unavailable; starting agy without metering")
		return runChild(ctx, command, args, env, dir, stdout, stderr)
	}
	defer m.Close()
	bundle, err := m.Bundle()
	if err != nil {
		_ = m.Close()
		fmt.Fprintln(stderr, "harnez-agy: metering proxy unavailable; starting agy without metering")
		return runChild(ctx, command, args, env, dir, stdout, stderr)
	}
	go m.Serve()
	env = setEnv(env, "HTTPS_PROXY", m.URL())
	env = setEnv(env, "https_proxy", m.URL())
	env = setEnv(env, "SSL_CERT_FILE", bundle)
	return runChild(ctx, command, args, env, dir, stdout, stderr)
}
func runChild(ctx context.Context, name string, args, env []string, dir string, out, errout io.Writer) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Env = env
	c.Dir = dir
	c.Stdout = out
	c.Stderr = errout
	return c.Run()
}
func setEnv(env []string, key, value string) []string {
	out := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, key+"=") {
			out = append(out, e)
		}
	}
	return append(out, key+"="+value)
}

func New(home string) (*Meter, error) {
	return newMeter(home, os.Getenv("HARNEZ_SESSION_ID"), "")
}

func newMeter(home, sessionID, promptID string) (*Meter, error) {
	dir := filepath.Join(home, ".harnez", "agymeter")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	cert, err := loadCA(dir)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	m := &Meter{listener: ln, cert: cert, logPath: filepath.Join(dir, "usage.jsonl"), debug: os.Getenv("HARNEZ_AGY_METER_DEBUG") == "1", sessionID: sessionID, promptID: promptID, lastQuota: make(map[string]Record)}
	if err := m.loadLastQuota(); err != nil {
		_ = ln.Close()
		return nil, err
	}
	m.server = &http.Server{Handler: http.HandlerFunc(m.handle), ReadHeaderTimeout: 10 * time.Second}
	return m, nil
}

func newPromptID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func envValue(env []string, name string) string {
	prefix := name + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func (m *Meter) URL() string  { return "http://" + m.listener.Addr().String() }
func (m *Meter) Serve()       { _ = m.server.Serve(m.listener) }
func (m *Meter) Close() error { return m.server.Close() }

func loadCA(dir string) (tls.Certificate, error) {
	keyPath, certPath := filepath.Join(dir, "ca.key"), filepath.Join(dir, "ca.crt")
	keyPEM, _ := os.ReadFile(keyPath)
	certPEM, _ := os.ReadFile(certPath)
	if b, _ := pem.Decode(certPEM); b != nil {
		if cert, e := x509.ParseCertificate(b.Bytes); e == nil && time.Now().Before(cert.NotAfter) && len(keyPEM) > 0 {
			if e = os.Chmod(keyPath, 0600); e != nil {
				return tls.Certificate{}, e
			}
			return tls.X509KeyPair(certPEM, keyPEM)
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "Harnez AGY Meter"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(5, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err = os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return tls.Certificate{}, err
	}
	if err = os.WriteFile(certPath, certPEM, 0644); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func (m *Meter) Bundle() (string, error) {
	var roots []byte
	for _, p := range []string{"/etc/pki/tls/certs/ca-bundle.crt", "/etc/ssl/certs/ca-certificates.crt", "/etc/ssl/cert.pem"} {
		if b, e := os.ReadFile(p); e == nil {
			roots = b
			break
		}
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("no system CA bundle found")
	}
	dir := filepath.Dir(m.logPath)
	ca, e := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if e != nil {
		return "", e
	}
	p := filepath.Join(dir, "roots.pem")
	if e = os.WriteFile(p, append(append([]byte{}, roots...), ca...), 0600); e != nil {
		return "", e
	}
	return p, nil
}

func (m *Meter) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}
	if !strings.EqualFold(strings.Split(r.Host, ":")[0], host) {
		m.debugf("CONNECT tunnel other host")
		m.tunnel(w, r)
		return
	}
	m.debugf("CONNECT allowlisted host")
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unavailable", 500)
		return
	}
	c, rw, e := hj.Hijack()
	if e != nil {
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = rw.Flush()
	tlsConn := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{m.cert}, GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) { return m.leaf(chi.ServerName) }})
	if e = tlsConn.Handshake(); e != nil {
		m.debugf("TLS handshake failed: %v", e)
		_ = c.Close()
		return
	}
	m.debugf("TLS handshake succeeded alpn=%s", tlsConn.ConnectionState().NegotiatedProtocol)
	_ = http.Serve(&singleConnListener{Conn: tlsConn}, http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { m.forward(w, q) }))
}
func (m *Meter) tunnel(w http.ResponseWriter, r *http.Request) {
	dst, e := net.DialTimeout("tcp", r.Host, 5*time.Second)
	if e != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = dst.Close()
		return
	}
	src, rw, e := hj.Hijack()
	if e != nil {
		_ = dst.Close()
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = rw.Flush()
	go func() { _, _ = io.Copy(dst, src); _ = dst.Close() }()
	_, _ = io.Copy(src, dst)
	_ = src.Close()
}
func (m *Meter) leaf(name string) (*tls.Certificate, error) {
	if name == "" {
		name = host
	}
	if cached, ok := m.leaves.Load(name); ok {
		return cached.(*tls.Certificate), nil
	}
	k, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return nil, e
	}
	parent, _ := x509.ParseCertificate(m.cert.Certificate[0])
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, tpl, parent, &k.PublicKey, m.cert.PrivateKey)
	if e != nil {
		return nil, e
	}
	leaf := &tls.Certificate{Certificate: [][]byte{der, m.cert.Certificate[0]}, PrivateKey: k}
	actual, _ := m.leaves.LoadOrStore(name, leaf)
	return actual.(*tls.Certificate), nil
}
func (m *Meter) forward(w http.ResponseWriter, r *http.Request) {
	m.debugf("intercepted request method=%s", r.Method)
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	var request map[string]any
	_ = json.Unmarshal(body, &request)
	model, _ := request["model"].(string)
	session, _ := request["sessionId"].(string)
	if session == "" {
		session, _ = request["conversationId"].(string)
	}
	conversation := session
	session = m.sessionID
	r.URL.Scheme = "https"
	r.URL.Host = host
	r.Host = host
	proxy := httputil.NewSingleHostReverseProxy((urlValue{scheme: "https", host: host}).URL())
	proxy.FlushInterval = -1
	proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	proxy.ModifyResponse = func(resp *http.Response) error {
		m.debugf("upstream response status=%d", resp.StatusCode)
		resp.Body = newResponseObserver(resp.Body, m, r.URL.Path, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Encoding"), model, session, conversation)
		return nil
	}
	proxy.ServeHTTP(w, r)
}
func (m *Meter) record(r Record) {
	b, e := json.Marshal(r)
	if e != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Kind == "quota" {
		if previous, ok := m.lastQuota[r.Bucket]; ok && sameQuota(previous, r) {
			return
		}
	}
	f, e := os.OpenFile(m.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		_, _ = f.Write(append(b, '\n'))
		_ = f.Close()
		if r.Kind == "quota" {
			m.lastQuota[r.Bucket] = r
		}
		m.debugf("record appended kind=%s", r.Kind)
	}
}

func sameQuota(a, b Record) bool {
	if a.Bucket != b.Bucket || a.Reset != b.Reset {
		return false
	}
	if a.Remaining == nil || b.Remaining == nil {
		return a.Remaining == nil && b.Remaining == nil
	}
	return *a.Remaining == *b.Remaining
}

func (m *Meter) loadLastQuota() error {
	f, err := os.Open(m.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		var row Record
		if json.Unmarshal(s.Bytes(), &row) == nil && row.Kind == "quota" && row.Bucket != "" {
			m.lastQuota[row.Bucket] = row
		}
	}
	return s.Err()
}

// ReadUsageRecords returns model-response rows recorded for session.
func ReadUsageRecords(path, session string) ([]Record, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	s := bufio.NewScanner(f)
	for s.Scan() {
		var row Record
		if json.Unmarshal(s.Bytes(), &row) == nil && row.Kind == "usage" && row.Session == session {
			out = append(out, row)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}

func (m *Meter) debugf(format string, args ...any) {
	if m.debug {
		log.Printf("agy-meter: "+format, args...)
	}
}
func num(v any) int64        { f, _ := v.(float64); return int64(f) }
func floatNum(v any) float64 { f, _ := v.(float64); return f }
func stringVal(v any) string { s, _ := v.(string); return s }

type responseObserver struct {
	io.ReadCloser
	chunks *asyncChunks
	done   chan struct{}
}

func newResponseObserver(body io.ReadCloser, m *Meter, path, contentType, encoding, model, session, conversation string) *responseObserver {
	return newObservedBody(body, func(src io.Reader) { m.parseResponse(src, path, contentType, encoding, model, session, conversation) }, func() { m.debugf("response side-reader dropped data after queue filled") })
}

func newObservedBody(body io.ReadCloser, parser func(io.Reader), dropped func()) *responseObserver {
	chunks := &asyncChunks{queue: make(chan []byte, 16), dropped: dropped}
	o := &responseObserver{ReadCloser: body, chunks: chunks, done: make(chan struct{})}
	go func() { defer close(o.done); parser(chunks) }()
	return o
}
func (o *responseObserver) Read(p []byte) (int, error) {
	n, e := o.ReadCloser.Read(p)
	if n > 0 {
		o.chunks.offer(p[:n])
	}
	if e != nil {
		o.chunks.finish()
	}
	return n, e
}
func (o *responseObserver) Close() error {
	o.chunks.finish()
	return o.ReadCloser.Close()
}

type asyncChunks struct {
	queue   chan []byte
	mu      sync.Mutex
	closed  bool
	dropped func()
	pending []byte
}

func (q *asyncChunks) offer(p []byte) {
	data := append([]byte(nil), p...)
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	select {
	case q.queue <- data:
	default:
		q.closed = true
		close(q.queue)
		if q.dropped != nil {
			q.dropped()
		}
	}
}
func (q *asyncChunks) finish() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.queue)
	}
}
func (q *asyncChunks) Read(p []byte) (int, error) {
	for len(q.pending) == 0 {
		chunk, ok := <-q.queue
		if !ok {
			return 0, io.EOF
		}
		q.pending = chunk
	}
	n := copy(p, q.pending)
	q.pending = q.pending[n:]
	return n, nil
}

func (m *Meter) parseResponse(raw io.Reader, path, contentType, encoding, model, session, conversation string) {
	var src io.Reader = raw
	if strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
		gz, err := gzip.NewReader(raw)
		if err != nil {
			m.debugf("gzip side-reader failed: %v", err)
			return
		}
		defer gz.Close()
		src = gz
	} else if encoding != "" && !strings.EqualFold(encoding, "identity") {
		m.debugf("unsupported content encoding for side-reader")
		return
	}
	var latest *Record
	quotas := map[string]Record{}
	consume := func(value any) {
		if strings.Contains(path, "streamGenerateContent") {
			walkUsage(value, func(u map[string]any) {
				r := Record{Time: time.Now().UTC(), Kind: "usage", Session: session, Conversation: conversation, PromptID: m.promptID, Model: model, Prompt: num(u["promptTokenCount"]), Candidates: num(u["candidatesTokenCount"]), Thoughts: num(u["thoughtsTokenCount"]), Cached: num(u["cachedContentTokenCount"]), Total: num(u["totalTokenCount"])}
				latest = &r
			})
		}
		if strings.Contains(path, "retrieveUserQuotaSummary") {
			walkQuota(value, func(r Record) { r.Session = m.sessionID; quotas[r.Bucket] = r })
		}
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		reader := bufio.NewReader(src)
		var event bytes.Buffer
		for {
			line, err := reader.ReadString('\n')
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				event.WriteString(strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
			}
			if trimmed == "" && event.Len() > 0 {
				var v any
				if json.Unmarshal(event.Bytes(), &v) == nil {
					consume(v)
				}
				event.Reset()
			}
			if err != nil {
				if event.Len() > 0 {
					var v any
					if json.Unmarshal(event.Bytes(), &v) == nil {
						consume(v)
					}
				}
				break
			}
		}
	} else if strings.Contains(path, "retrieveUserQuotaSummary") {
		var v any
		if err := json.NewDecoder(src).Decode(&v); err == nil {
			consume(v)
		} else {
			m.debugf("quota JSON parse failed: %v", err)
		}
	}
	m.debugf("response parse complete usage=%t quota_buckets=%d", latest != nil, len(quotas))
	if latest != nil {
		m.record(*latest)
	}
	for _, r := range quotas {
		m.record(r)
	}
}

func walkUsage(v any, found func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		for k, value := range x {
			if k == "usageMetadata" {
				if u, ok := value.(map[string]any); ok {
					found(u)
				}
			}
			walkUsage(value, found)
		}
	case []any:
		for _, value := range x {
			walkUsage(value, found)
		}
	}
}
func walkQuota(v any, found func(Record)) {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["bucketId"].(string); ok {
			remaining := floatNum(x["remainingFraction"])
			found(Record{Time: time.Now().UTC(), Kind: "quota", Bucket: id, Remaining: &remaining, Reset: stringVal(x["resetTime"])})
		}
		for _, value := range x {
			walkQuota(value, found)
		}
	case []any:
		for _, value := range x {
			walkQuota(value, found)
		}
	}
}

type singleConnListener struct{ net.Conn }

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.Conn == nil {
		return nil, io.EOF
	}
	c := l.Conn
	l.Conn = nil
	return c, nil
}
func (*singleConnListener) Close() error   { return nil }
func (*singleConnListener) Addr() net.Addr { return nil }

type urlValue struct{ scheme, host string }

func (u urlValue) URL() *url.URL { return &url.URL{Scheme: u.scheme, Host: u.host} }
