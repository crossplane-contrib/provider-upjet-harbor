// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crossplane/upjet/v2/pkg/diffserver"
	harborclient "github.com/goharbor/terraform-provider-harbor/client"
)

// harborHost is the host of offlineTestURL, which the tripwire must see a
// request for when nothing stands in the request's way.
const harborHost = "harbor.example.com"

// realDefaultTransport is net/http's own default transport, captured during
// package variable initialisation, before any test can call
// EnableOfflineHTTPTransport, whose sync.Once swap cannot be undone. Holding
// on to it lets the positive control put the real transport back regardless
// of what ran first.
var realDefaultTransport = http.DefaultTransport

// egress is the tripwire armed by TestMain, shared by every test in the
// package. Tests that read it must not run in parallel with one another.
var egress *proxyRecorder

// proxyRecorder is an HTTP proxy that records every request routed through it
// and forwards none of them.
//
// The Harbor client sends every request through a fresh http.Client with no
// transport of its own, which means http.DefaultTransport, and that transport
// honours HTTPS_PROXY. Pointing the proxy at a recorder therefore puts a
// tripwire on the process's real network boundary: nothing is stubbed or
// injected, and a request cannot reach Harbor without being recorded here
// first. The same tripwire works unchanged against the compiled provider
// binary.
type proxyRecorder struct {
	url string

	mu   sync.Mutex
	seen []string
}

func (p *proxyRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CONNECT carries its target in Host; an absolute-form proxy request
	// carries it in URL.
	target := r.Host
	if target == "" && r.URL != nil {
		target = r.URL.Host
	}
	p.mu.Lock()
	p.seen = append(p.seen, r.Method+" "+target)
	p.mu.Unlock()
	http.Error(w, "blocked by the egress tripwire", http.StatusBadGateway)
}

func (p *proxyRecorder) records() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.seen...)
}

func (p *proxyRecorder) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = nil
}

// reached reports whether any recorded request targeted host.
func (p *proxyRecorder) reached(host string) bool {
	for _, r := range p.records() {
		if strings.Contains(r, host) {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot listen for the egress tripwire: %v\n", err)
		return 1
	}
	defer l.Close() //nolint:errcheck // the process is exiting

	egress = &proxyRecorder{url: "http://" + l.Addr().String()}
	srv := &http.Server{Handler: egress, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(l)   //nolint:errcheck // Serve always returns a non-nil error
	defer srv.Close() //nolint:errcheck // the process is exiting

	// net/http resolves the proxy environment exactly once per process, so
	// this has to happen before anything in the package makes a request.
	os.Setenv("HTTP_PROXY", egress.url)  //nolint:errcheck // must outlive any single test, so t.Setenv will not do
	os.Setenv("HTTPS_PROXY", egress.url) //nolint:errcheck // must outlive any single test, so t.Setenv will not do
	os.Setenv("NO_PROXY", "")            //nolint:errcheck // must outlive any single test, so t.Setenv will not do

	return m.Run()
}

// TestOfflineRequestDoesNotEgress asserts that a request the offline setup's
// Harbor client sends is refused inside the process: at once, with the error
// the diff server reports as DIFF_COMPUTATION_NOT_SUPPORTED, and without
// reaching the network boundary.
func TestOfflineRequestDoesNotEgress(t *testing.T) {
	EnableOfflineHTTPTransport()
	egress.reset()

	c := offlineHarborClient(t)
	start := time.Now()
	_, _, _, err := c.SendRequest(http.MethodGet, "/projects", nil, http.StatusOK)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the offline Harbor client to refuse the request")
	}
	if !diffserver.IsDiffComputationNotSupportedError(err) {
		t.Errorf("expected a diff computation not supported error, got: %v", err)
	}
	// The guard's value is that it fails at once, where an unreachable Harbor
	// would make the client wait for a connection timeout.
	if elapsed > 10*time.Second {
		t.Errorf("expected the blocked request to fail immediately, took %s", elapsed)
	}
	if got := egress.records(); len(got) > 0 {
		t.Errorf("the offline Harbor client reached the network: %v", got)
	}
}

// TestEgressTripwireObservesUnguardedRequest is the positive control for
// TestOfflineRequestDoesNotEgress. That test is only meaningful if the
// tripwire can see the Harbor client's traffic at all, so this one puts
// net/http's real default transport back and sends the same request: with
// nothing in the way, it reaches the network boundary and the tripwire
// records it.
func TestEgressTripwireObservesUnguardedRequest(t *testing.T) {
	EnableOfflineHTTPTransport()
	t.Cleanup(func() { http.DefaultTransport = offlineTransport{} })
	http.DefaultTransport = realDefaultTransport
	egress.reset()

	c := offlineHarborClient(t)
	// Expected to fail: the tripwire refuses the request. What matters is
	// that the request was seen leaving, not how the client reports it.
	_, _, _, _ = c.SendRequest(http.MethodGet, "/projects", nil, http.StatusOK)

	if !egress.reached(harborHost) {
		t.Errorf("expected the unguarded request to reach %s, recorded: %v", harborHost, egress.records())
	}
}

func offlineHarborClient(t *testing.T) *harborclient.Client {
	t.Helper()
	ps := buildOfflineHarborSetup(offlineTestSpec(offlineTestURL))
	c, ok := ps.Meta.(*harborclient.Client)
	if !ok {
		t.Fatalf("terraform.Setup.Meta is %T, want *client.Client", ps.Meta)
	}
	return c
}
