package romm

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestIsMDNSHost(t *testing.T) {
	for host, want := range map[string]bool{
		"sado.local": true, "SADO.LOCAL.": true, "rom.arnaut.me": false, "local": false, "10.0.0.1": false,
	} {
		if got := isMDNSHost(host); got != want {
			t.Errorf("isMDNSHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestParseMDNSAnswersCollectsAllAddresses(t *testing.T) {
	name := dnsmessage.MustNewName("sado.local.")
	other := dnsmessage.MustNewName("other.local.")
	a := func(n dnsmessage.Name, ip [4]byte) dnsmessage.Resource {
		return dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{Name: n, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 120},
			Body:   &dnsmessage.AResource{A: ip},
		}
	}
	msg := dnsmessage.Message{
		Header:      dnsmessage.Header{Response: true, Authoritative: true},
		Answers:     []dnsmessage.Resource{a(name, [4]byte{10, 42, 42, 228}), a(other, [4]byte{1, 2, 3, 4})},
		Additionals: []dnsmessage.Resource{a(name, [4]byte{10, 42, 42, 245})},
	}
	packet, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}
	got := parseMDNSAnswers(packet, name)
	want := []netip.Addr{netip.MustParseAddr("10.42.42.228"), netip.MustParseAddr("10.42.42.245")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestQueryIsWellFormed(t *testing.T) {
	q, err := buildMDNSQuery(dnsmessage.MustNewName("sado.local."))
	if err != nil {
		t.Fatal(err)
	}
	var p dnsmessage.Parser
	if _, err := p.Start(q); err != nil {
		t.Fatal(err)
	}
	question, err := p.Question()
	if err != nil || question.Type != dnsmessage.TypeA || question.Name.String() != "sado.local." {
		t.Fatalf("question = %+v, %v", question, err)
	}
}

func heartbeatServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/heartbeat" {
			_, _ = w.Write([]byte(`{"SYSTEM":{"VERSION":"test"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewWithFallback(t *testing.T) {
	public := heartbeatServer(t)
	local := heartbeatServer(t)

	// A port nothing listens on.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := "http://" + l.Addr().String()
	l.Close()

	ctx := context.Background()
	if c := NewWithFallback(ctx, public.URL, local.URL, "t"); !c.Local || c.BaseURL != local.URL {
		t.Errorf("reachable local: got %s local=%v", c.BaseURL, c.Local)
	}
	start := time.Now()
	if c := NewWithFallback(ctx, public.URL, dead, "t"); c.Local || c.BaseURL != public.URL {
		t.Errorf("dead local: got %s local=%v", c.BaseURL, c.Local)
	}
	if time.Since(start) > localProbeTimeout+time.Second {
		t.Errorf("fallback took %v", time.Since(start))
	}
	if c := NewWithFallback(ctx, public.URL, "", "t"); c.Local || c.BaseURL != public.URL {
		t.Errorf("no local: got %s local=%v", c.BaseURL, c.Local)
	}
}

// TestLookupMDNSLive resolves a real .local host on the network. It only runs
// with ROMMATE_MDNS_HOST set, e.g. ROMMATE_MDNS_HOST=sado.local.
func TestLookupMDNSLive(t *testing.T) {
	host := os.Getenv("ROMMATE_MDNS_HOST")
	if host == "" {
		t.Skip("set ROMMATE_MDNS_HOST to test against a real .local host")
	}
	addrs, err := lookupMDNS(context.Background(), host)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s → %v", host, addrs)
}

func TestRacePicksReachableAddress(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())

	// A closed port refuses immediately.
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	_, closedPort, _ := net.SplitHostPort(closed.Addr().String())
	closed.Close()

	d := &dialer{}
	addrs := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	conn, err := d.race(context.Background(), "tcp", "sado.local", port, addrs)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if a, ok := d.pref.get("sado.local"); !ok || a.String() != "127.0.0.1" {
		t.Fatalf("winner not remembered: %v %v", a, ok)
	}
	if _, err := d.race(context.Background(), "tcp", "sado.local", closedPort, addrs); err == nil {
		t.Fatal("expected an error when no address accepts")
	}
}
