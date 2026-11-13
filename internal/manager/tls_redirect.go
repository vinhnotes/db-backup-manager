package manager

import (
	"bufio"
	"net"
	"net/http"
	"sync"
	"time"
)

// SplitTLS lets HTTPS and plain HTTP share one port. Each connection is
// sorted by its first byte: a TLS handshake (0x16) goes to the tls listener,
// anything else to the plain one, which should only redirect to HTTPS.
func SplitTLS(l net.Listener) (tlsL, plainL net.Listener) {
	s := &splitter{Listener: l, tls: make(chan net.Conn), plain: make(chan net.Conn), done: make(chan struct{})}
	go s.run()
	return &splitChild{s, s.tls}, &splitChild{s, s.plain}
}

type splitter struct {
	net.Listener
	tls, plain chan net.Conn
	done       chan struct{}
	once       sync.Once
	err        error
}

func (s *splitter) run() {
	for {
		c, err := s.Listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			s.closeWith(err)
			return
		}
		go s.sort(c)
	}
}

// sort runs per connection so a slow client cannot block other accepts.
func (s *splitter) sort(c net.Conn) {
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(c)
	first, err := r.Peek(1)
	_ = c.SetReadDeadline(time.Time{})
	if err != nil {
		c.Close()
		return
	}
	ch := s.plain
	if first[0] == 0x16 {
		ch = s.tls
	}
	select {
	case ch <- &peekedConn{c, r}:
	case <-s.done:
		c.Close()
	}
}

func (s *splitter) close() { s.closeWith(nil) }

// closeWith records why accepting stopped (nil after Close) before waking
// the child listeners, so they can read err without a race.
func (s *splitter) closeWith(err error) {
	s.once.Do(func() {
		s.err = err
		close(s.done)
		s.Listener.Close()
	})
}

type splitChild struct {
	s  *splitter
	ch chan net.Conn
}

func (c *splitChild) Accept() (net.Conn, error) {
	select {
	case conn := <-c.ch:
		return conn, nil
	case <-c.s.done:
		if c.s.err != nil {
			return nil, c.s.err
		}
		return nil, net.ErrClosed
	}
}

// Close closes the shared port, so both servers stop accepting.
func (c *splitChild) Close() error   { c.s.close(); return nil }
func (c *splitChild) Addr() net.Addr { return c.s.Addr() }

type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// RedirectToHTTPS sends plain HTTP requests to the same host and port over
// HTTPS. 307 keeps the method and is not cached for good, so turning TLS off
// later does not leave browsers stuck on https://.
func RedirectToHTTPS() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "" {
			http.Error(w, "Use https://", http.StatusBadRequest)
			return
		}
		w.Header().Set("Connection", "close")
		http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusTemporaryRedirect) // #nosec G710 -- redirects the client to the host it requested, only switching to https.
	})
}
