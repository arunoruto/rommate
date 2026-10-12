package romm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// rommate is built without CGO, so on Linux Go uses its own DNS resolver,
// which does not speak mDNS: "sado.local" fails even though the system (via
// Avahi) can resolve it. This file adds a small mDNS client for .local names.
//
// Like Avahi and macOS, it listens on the mDNS port 5353 (shared with the
// system's responder via SO_REUSEADDR/SO_REUSEPORT), joins the IPv4 and IPv6
// mDNS groups on every interface, sends one query per interface and collects
// the A records that come back. Both families matter: some responders only
// answer over IPv6 (ff02::fb). If port 5353 cannot be shared, it falls back to
// a "legacy unicast" query from an ephemeral port (RFC 6762 §6.7). All answers
// within the timeout are kept, because hosts with Docker or several interfaces
// announce many addresses and only some of them may be reachable.

var (
	mdnsGroup4 = netip.MustParseAddrPort("224.0.0.251:5353")
	mdnsGroup6 = netip.MustParseAddr("ff02::fb")
)

const mdnsPort = 5353

// mdnsTimeout is how long to collect answers.
const mdnsTimeout = 500 * time.Millisecond

// isMDNSHost reports whether host is a multicast-DNS name.
func isMDNSHost(host string) bool {
	return strings.HasSuffix(strings.TrimSuffix(strings.ToLower(host), "."), ".local")
}

// lookupMDNS resolves a .local host name to its IPv4 addresses.
func lookupMDNS(ctx context.Context, host string) ([]netip.Addr, error) {
	name, err := dnsmessage.NewName(strings.TrimSuffix(host, ".") + ".")
	if err != nil {
		return nil, err
	}
	query, err := buildMDNSQuery(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(mdnsTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	ifaces := multicastInterfaces()
	var conns []*net.UDPConn
	if c := openMDNS4(ctx, ifaces); c != nil {
		conns = append(conns, c)
	}
	if c := openMDNS6(ctx, ifaces); c != nil {
		conns = append(conns, c)
	}
	if len(conns) == 0 {
		return nil, errors.New("mDNS: no usable network interface")
	}

	answers := make(chan []netip.Addr, 64)
	var wg sync.WaitGroup
	for _, c := range conns {
		defer c.Close()
		_ = c.SetDeadline(deadline)
		sendQuery(c, query, ifaces)
		wg.Add(1)
		go func(c *net.UDPConn) {
			defer wg.Done()
			buf := make([]byte, 9000)
			for {
				n, _, err := c.ReadFromUDPAddrPort(buf)
				if err != nil {
					return // deadline reached
				}
				if a := parseMDNSAnswers(buf[:n], name); len(a) > 0 {
					answers <- a
				}
			}
		}(c)
	}
	go func() { wg.Wait(); close(answers) }()

	seen := map[netip.Addr]bool{}
	var addrs []netip.Addr
	for batch := range answers {
		for _, a := range batch {
			if !seen[a] {
				seen[a] = true
				addrs = append(addrs, a)
			}
		}
	}
	if len(addrs) == 0 {
		return nil, &net.DNSError{Err: "no mDNS answer", Name: host, IsNotFound: true}
	}
	return addrs, nil
}

func multicastInterfaces() []net.Interface {
	all, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.Interface
	for _, i := range all {
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagMulticast != 0 && i.Flags&net.FlagLoopback == 0 {
			out = append(out, i)
		}
	}
	return out
}

// openMDNS4 opens an IPv4 socket on port 5353 joined to 224.0.0.251, or an
// ephemeral-port socket for legacy unicast queries if 5353 can't be shared.
func openMDNS4(ctx context.Context, ifaces []net.Interface) *net.UDPConn {
	lc := net.ListenConfig{Control: reusePort}
	if pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf("0.0.0.0:%d", mdnsPort)); err == nil {
		conn := pc.(*net.UDPConn)
		p := ipv4.NewPacketConn(conn)
		group := net.UDPAddrFromAddrPort(mdnsGroup4)
		for i := range ifaces {
			_ = p.JoinGroup(&ifaces[i], group)
		}
		return conn
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil
	}
	return conn
}

// openMDNS6 opens an IPv6 socket on port 5353 joined to ff02::fb.
func openMDNS6(ctx context.Context, ifaces []net.Interface) *net.UDPConn {
	lc := net.ListenConfig{Control: reusePort}
	pc, err := lc.ListenPacket(ctx, "udp6", fmt.Sprintf("[::]:%d", mdnsPort))
	if err != nil {
		return nil
	}
	conn := pc.(*net.UDPConn)
	p := ipv6.NewPacketConn(conn)
	group := &net.UDPAddr{IP: mdnsGroup6.AsSlice()}
	for i := range ifaces {
		_ = p.JoinGroup(&ifaces[i], group)
	}
	return conn
}

// sendQuery sends the query out of every interface: the default route alone
// is not enough, because with a VPN such as Tailscale it points into the
// tunnel instead of the LAN.
func sendQuery(conn *net.UDPConn, query []byte, ifaces []net.Interface) {
	if conn.LocalAddr().(*net.UDPAddr).IP.To4() != nil {
		p := ipv4.NewPacketConn(conn)
		_ = p.SetMulticastTTL(255)
		group := net.UDPAddrFromAddrPort(mdnsGroup4)
		for i := range ifaces {
			if p.SetMulticastInterface(&ifaces[i]) == nil {
				_, _ = p.WriteTo(query, nil, group)
			}
		}
		return
	}
	p := ipv6.NewPacketConn(conn)
	_ = p.SetMulticastHopLimit(255)
	for i := range ifaces {
		if p.SetMulticastInterface(&ifaces[i]) == nil {
			_, _ = p.WriteTo(query, nil, &net.UDPAddr{IP: mdnsGroup6.AsSlice(), Port: mdnsPort, Zone: ifaces[i].Name})
		}
	}
}

func buildMDNSQuery(name dnsmessage.Name) ([]byte, error) {
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 0},
		Questions: []dnsmessage.Question{{
			Name:  name,
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}},
	}
	return msg.Pack()
}

// parseMDNSAnswers returns the A records for name in an mDNS response.
func parseMDNSAnswers(packet []byte, name dnsmessage.Name) []netip.Addr {
	var p dnsmessage.Parser
	h, err := p.Start(packet)
	if err != nil || !h.Response {
		return nil
	}
	if err := p.SkipAllQuestions(); err != nil {
		return nil
	}
	var out []netip.Addr
	collect := func(next func() (dnsmessage.ResourceHeader, error), skip func() error) {
		for {
			rh, err := next()
			if err != nil {
				return
			}
			if rh.Type == dnsmessage.TypeA && strings.EqualFold(rh.Name.String(), name.String()) {
				if a, err := p.AResource(); err == nil {
					out = append(out, netip.AddrFrom4(a.A))
				}
				continue
			}
			if err := skip(); err != nil {
				return
			}
		}
	}
	collect(p.AnswerHeader, p.SkipAnswer)
	if err := p.SkipAllAuthorities(); err == nil {
		collect(p.AdditionalHeader, p.SkipAdditional)
	}
	return out
}

// mdnsCache remembers resolved .local hosts briefly so a session does not
// query the network for every request.
type mdnsCache struct {
	mu      sync.Mutex
	entries map[string]mdnsEntry
}

type mdnsEntry struct {
	addrs   []netip.Addr
	expires time.Time
}

const mdnsCacheTTL = 2 * time.Minute

func (c *mdnsCache) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	c.mu.Lock()
	if e, ok := c.entries[host]; ok && time.Now().Before(e.expires) {
		c.mu.Unlock()
		return e.addrs, nil
	}
	c.mu.Unlock()

	addrs, err := lookupMDNS(ctx, host)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]mdnsEntry{}
	}
	c.entries[host] = mdnsEntry{addrs: addrs, expires: time.Now().Add(mdnsCacheTTL)}
	c.mu.Unlock()
	return addrs, nil
}

// preferred remembers which address of a .local host last connected, so
// later connections go straight to it.
type preferred struct {
	mu   sync.Mutex
	addr map[string]netip.Addr
}

func (p *preferred) get(host string) (netip.Addr, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.addr[host]
	return a, ok
}

func (p *preferred) remember(host string, a netip.Addr) {
	p.mu.Lock()
	if p.addr == nil {
		p.addr = map[string]netip.Addr{}
	}
	p.addr[host] = a
	p.mu.Unlock()
}

// dialer connects to .local hosts via mDNS and everything else normally.
type dialer struct {
	net.Dialer
	cache mdnsCache
	pref  preferred
}

// perAddressTimeout bounds each connection attempt to one address.
const perAddressTimeout = 2 * time.Second

func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || !isMDNSHost(host) {
		return d.Dialer.DialContext(ctx, network, address)
	}
	addrs, err := d.cache.lookup(ctx, host)
	if err != nil {
		// Let the system resolver try (it handles .local on macOS, or when
		// the name is also in /etc/hosts or local DNS).
		return d.Dialer.DialContext(ctx, network, address)
	}
	if conn, ok := d.preferredConn(ctx, network, host, port); ok {
		return conn, nil
	}
	return d.race(ctx, network, host, port, addrs)
}

// preferredConn tries the address that worked last time on its own first.
func (d *dialer) preferredConn(ctx context.Context, network, host, port string) (net.Conn, bool) {
	a, ok := d.pref.get(host)
	if !ok {
		return nil, false
	}
	attempt, cancel := context.WithTimeout(ctx, perAddressTimeout)
	defer cancel()
	conn, err := d.Dialer.DialContext(attempt, network, netip.AddrPortFrom(a, portNumber(port)).String())
	return conn, err == nil
}

// race dials every address at once and keeps the first connection. Hosts with
// Docker or Kubernetes often announce addresses that are not reachable from
// the LAN; trying them one by one would take seconds per dead address.
func (d *dialer) race(ctx context.Context, network, host, port string, addrs []netip.Addr) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, perAddressTimeout)
	defer cancel()
	type result struct {
		conn net.Conn
		addr netip.Addr
		err  error
	}
	results := make(chan result, len(addrs))
	for _, a := range addrs {
		go func(a netip.Addr) {
			conn, err := d.Dialer.DialContext(ctx, network, netip.AddrPortFrom(a, portNumber(port)).String())
			results <- result{conn, a, err}
		}(a)
	}
	var winner net.Conn
	var errs []error
	for range addrs {
		r := <-results
		switch {
		case r.err != nil:
			errs = append(errs, r.err)
		case winner == nil:
			winner = r.conn
			d.pref.remember(host, r.addr)
			cancel() // stop the others
		default:
			r.conn.Close() // a slower success
		}
	}
	if winner != nil {
		return winner, nil
	}
	return nil, errors.Join(errs...)
}

func portNumber(p string) uint16 {
	n, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(n)
}
