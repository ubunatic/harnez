// Package agymeter implements the opt-in, per-process Antigravity usage proxy.
package agymeter

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const host = "daily-cloudcode-pa.googleapis.com"

type Record struct {
	Time       time.Time `json:"time"`
	Kind       string    `json:"kind"`
	Session    string    `json:"session,omitempty"`
	Model      string    `json:"model,omitempty"`
	Prompt     int64     `json:"promptTokenCount,omitempty"`
	Candidates int64     `json:"candidatesTokenCount,omitempty"`
	Thoughts   int64     `json:"thoughtsTokenCount,omitempty"`
	Cached     int64     `json:"cachedContentTokenCount,omitempty"`
	Total      int64     `json:"totalTokenCount,omitempty"`
	Bucket     string    `json:"bucketId,omitempty"`
	Remaining  float64   `json:"remainingFraction,omitempty"`
	Reset      string    `json:"resetTime,omitempty"`
}

type Meter struct {
	listener net.Listener
	server   *http.Server
	cert     tls.Certificate
	logPath  string
	mu       sync.Mutex
}

// Run starts a private meter for the lifetime of the child command. Failure to
// initialize metering is fail-open: the requested program still runs plainly.
func Run(ctx context.Context, home string, command string, args []string, stdout, stderr io.Writer) error {
	m, err := New(home)
	if err != nil {
		fmt.Fprintln(stderr, "harnez-agy: metering proxy unavailable; starting agy without metering")
		return runChild(ctx, command, args, os.Environ(), stdout, stderr)
	}
	defer m.Close()
	bundle, err := m.Bundle()
	if err != nil {
		_ = m.Close()
		fmt.Fprintln(stderr, "harnez-agy: metering proxy unavailable; starting agy without metering")
		return runChild(ctx, command, args, os.Environ(), stdout, stderr)
	}
	go m.Serve()
	env := setEnv(os.Environ(), "HTTPS_PROXY", m.URL())
	env = setEnv(env, "https_proxy", m.URL())
	env = setEnv(env, "SSL_CERT_FILE", bundle)
	return runChild(ctx, command, args, env, stdout, stderr)
}
func runChild(ctx context.Context, name string, args, env []string, out, errout io.Writer) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Env = env
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
	m := &Meter{listener: ln, cert: cert, logPath: filepath.Join(dir, "usage.jsonl")}
	m.server = &http.Server{Handler: http.HandlerFunc(m.handle), ReadHeaderTimeout: 10 * time.Second}
	return m, nil
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
		m.tunnel(w, r)
		return
	}
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
		_ = c.Close()
		return
	}
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
	return &tls.Certificate{Certificate: [][]byte{der, m.cert.Certificate[0]}, PrivateKey: k}, nil
}
func (m *Meter) forward(w http.ResponseWriter, r *http.Request) {
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
	r.URL.Scheme = "https"
	r.URL.Host = host
	r.Host = host
	proxy := httputil.NewSingleHostReverseProxy((urlValue{scheme: "https", host: host}).URL())
	proxy.FlushInterval = -1
	proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Body = &responseObserver{ReadCloser: resp.Body, process: func(b []byte) { m.observe(r, b, model, session) }}
		return nil
	}
	proxy.ServeHTTP(w, r)
}
func (m *Meter) observe(r *http.Request, b []byte, model, session string) {
	path := ""
	if r != nil && r.URL != nil {
		path = r.URL.Path
	}
	for _, line := range bytes.Split(b, []byte("\n")) {
		line = bytes.TrimSpace(line)
		payload := line
		if bytes.HasPrefix(line, []byte("data:")) {
			payload = bytes.TrimSpace(line[5:])
		}
		var v map[string]any
		if json.Unmarshal(payload, &v) != nil {
			continue
		}
		if u, ok := v["usageMetadata"].(map[string]any); ok {
			m.record(Record{Time: time.Now().UTC(), Kind: "usage", Model: model, Session: session, Prompt: num(u["promptTokenCount"]), Candidates: num(u["candidatesTokenCount"]), Thoughts: num(u["thoughtsTokenCount"]), Cached: num(u["cachedContentTokenCount"]), Total: num(u["totalTokenCount"])})
		}
		if strings.Contains(path, "retrieveUserQuotaSummary") || path == "" {
			var walk func(any)
			walk = func(x any) {
				switch y := x.(type) {
				case map[string]any:
					if id, ok := y["bucketId"].(string); ok {
						m.record(Record{Time: time.Now().UTC(), Kind: "quota", Bucket: id, Remaining: floatNum(y["remainingFraction"]), Reset: stringVal(y["resetTime"])})
					}
					for _, z := range y {
						walk(z)
					}
				case []any:
					for _, z := range y {
						walk(z)
					}
				}
			}
			walk(v)
		}
	}
}
func (m *Meter) record(r Record) {
	b, e := json.Marshal(r)
	if e != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, e := os.OpenFile(m.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		_, _ = f.Write(append(b, '\n'))
		_ = f.Close()
	}
}
func requestSession(r *http.Request) string {
	for _, k := range []string{"sessionId", "conversationId"} {
		if s := r.URL.Query().Get(k); s != "" {
			return s
		}
	}
	return ""
}
func num(v any) int64        { f, _ := v.(float64); return int64(f) }
func floatNum(v any) float64 { f, _ := v.(float64); return f }
func stringVal(v any) string { s, _ := v.(string); return s }

type observed struct {
	io.ReadCloser
	fn func([]byte)
}

func (o *observed) Read(p []byte) (int, error) {
	n, e := o.ReadCloser.Read(p)
	if n > 0 {
		o.fn(append([]byte(nil), p[:n]...))
	}
	return n, e
}

type responseObserver struct {
	io.ReadCloser
	process func([]byte)
	once    sync.Once
	pending []byte
}

func (o *responseObserver) Read(p []byte) (int, error) {
	n, e := o.ReadCloser.Read(p)
	if n > 0 {
		o.pending = append(o.pending, p[:n]...)
		for {
			i := bytes.IndexByte(o.pending, '\n')
			if i < 0 {
				break
			}
			o.process(append([]byte(nil), o.pending[:i+1]...))
			o.pending = append(o.pending[:0], o.pending[i+1:]...)
		}
	}
	if e == io.EOF && len(o.pending) > 0 {
		o.process(append([]byte(nil), o.pending...))
		o.pending = nil
	}
	return n, e
}
func (o *responseObserver) Close() error { o.once.Do(func() { _ = o.ReadCloser.Close() }); return nil }

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
