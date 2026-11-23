package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"log"
	"maps"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type proxyStats struct {
	mu           sync.Mutex
	ConnectCount map[string]uint64 `json:"connectCount"`
	BytesCopied  atomic.Uint64     `json:"bytesCopied"`
}

func (s *proxyStats) recordConnect(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConnectCount[target]++
}

type countingWriter struct {
	w       io.Writer
	counter *atomic.Uint64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 {
		cw.counter.Add(uint64(n))
	}
	return n, err
}

func main() {
	var (
		proxyAddr = flag.String("proxy-listen", ":8443", "TLS listen address for HTTPS forward proxy")
		statsAddr = flag.String("stats-listen", ":8080", "HTTP listen address for /stats endpoint")
		certFile  = flag.String("tls-cert-file", "/etc/proxy-tls/tls.crt", "Path to proxy TLS server certificate")
		keyFile   = flag.String("tls-key-file", "/etc/proxy-tls/tls.key", "Path to proxy TLS server key")
	)
	flag.Parse()

	stats := &proxyStats{
		ConnectCount: make(map[string]uint64),
	}

	// 1. Expose /stats endpoint on plain HTTP for E2E test assertions
	statsMux := http.NewServeMux()
	statsMux.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		stats.mu.Lock()
		connectCountCopy := maps.Clone(stats.ConnectCount)
		stats.mu.Unlock()

		snapshot := map[string]any{
			"connectCount": connectCountCopy,
			"bytesCopied":  stats.BytesCopied.Load(),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot)
	})
	go func() {
		log.Printf("Starting stats server on %s", *statsAddr)
		if err := http.ListenAndServe(*statsAddr, statsMux); err != nil {
			log.Fatalf("stats server failed: %v", err)
		}
	}()

	// 2. HTTPS Forward Proxy handler supporting HTTP CONNECT
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT method is supported", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("CONNECT request for %s from %s", r.Host, r.RemoteAddr)
		dstConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
		if err != nil {
			log.Printf("Failed to dial target %s: %v", r.Host, err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer func() { _ = dstConn.Close() }()

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}

		// Hijack the connection to take raw control of the underlying stream.
		// Note: We deliberately do NOT call w.WriteHeader(http.StatusOK) prior to Hijack().
		// In Go's net/http, calling WriteHeader without a Content-Length causes the chunk-writer
		// to emit "Transfer-Encoding: chunked", violating RFC 9110 §9.3.6 (CONNECT tunnel
		// responses MUST NOT send Transfer-Encoding or Content-Length headers).
		// Instead, we hijack first and write the standard RFC "200 Connection Established"
		// response directly to the client's buffered writer and flush it before tunneling.
		clientConn, brw, err := hijacker.Hijack()
		if err != nil {
			log.Printf("Hijack failed: %v", err)
			return
		}

		if _, err := brw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			log.Printf("Failed to write 200 to client: %v", err)
			_ = clientConn.Close()
			return
		}
		if err := brw.Flush(); err != nil {
			log.Printf("Failed to flush 200 to client: %v", err)
			_ = clientConn.Close()
			return
		}

		stats.recordConnect(r.Host)

		// Close both connections when either stream terminates to ensure opposing io.Copy unblocks
		closeBoth := sync.OnceFunc(func() {
			_ = dstConn.Close()
			_ = clientConn.Close()
		})
		defer closeBoth()

		var wg sync.WaitGroup
		wg.Add(2)
		pipe := func(dst io.Writer, src io.Reader) {
			defer wg.Done()
			defer closeBoth()
			_, _ = io.Copy(dst, src)
		}

		// Wrap destinations with countingWriter to record transferred bytes in real-time
		// rather than waiting for persistent Keep-Alive connections to terminate.
		cwDst := &countingWriter{w: dstConn, counter: &stats.BytesCopied}
		cwClient := &countingWriter{w: clientConn, counter: &stats.BytesCopied}

		// brw.Reader preserves any pre-buffered client payload received during header processing
		go pipe(cwDst, brw.Reader)
		go pipe(cwClient, dstConn)
		wg.Wait()
	})

	server := &http.Server{
		Addr:              *proxyAddr,
		Handler:           proxyHandler,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	log.Printf("Starting HTTPS forward proxy on %s", *proxyAddr)
	log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
}
