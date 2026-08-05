// Package main implements a minimal DNS server for CRC wildcard resolution.
//
// It resolves api.crc.testing and *.apps-crc.testing to the host-gateway IP
// (obtained from the "crc-host" /etc/hosts entry injected by --add-host),
// and forwards everything else to the upstream nameserver from /etc/resolv.conf.
package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

func main() {
	upstream := findUpstream()
	targetIP := lookupHostIP("crc-host")

	log.Printf("upstream nameserver: %s", upstream)
	log.Printf("CRC target IP: %s", targetIP)

	handler := &crcHandler{
		upstream: upstream,
		targetIP: targetIP,
	}

	udpServer := &dns.Server{Addr: ":53", Net: "udp", Handler: handler}
	tcpServer := &dns.Server{Addr: ":53", Net: "tcp", Handler: handler}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := udpServer.ListenAndServe(); err != nil {
			log.Fatalf("UDP server failed: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Fatalf("TCP server failed: %v", err)
		}
	}()

	log.Println("DNS server listening on :53 (UDP+TCP)")
	wg.Wait()
}

type crcHandler struct {
	upstream string
	targetIP net.IP
}

func (h *crcHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	name := strings.ToLower(q.Name)

	if !isCRCName(name) {
		h.forward(w, r)
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	switch q.Qtype {
	case dns.TypeA:
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: append(net.IP(nil), h.targetIP...),
		})
	case dns.TypeAAAA:
		// Return empty NOERROR (no AAAA records available).
	default:
		// Return empty NOERROR for other types on matched names.
	}

	w.WriteMsg(m)
}

// isCRCName returns true for api.crc.testing. and *.apps-crc.testing. (FQDN with trailing dot).
func isCRCName(name string) bool {
	if name == "api.crc.testing." {
		return true
	}
	if strings.HasSuffix(name, ".apps-crc.testing.") {
		return true
	}
	return false
}

func (h *crcHandler) forward(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		c.Net = "tcp"
	}
	resp, _, err := c.Exchange(r, h.upstream)
	if err != nil {
		log.Printf("forward error: %v", err)
		dns.HandleFailed(w, r)
		return
	}
	w.WriteMsg(resp)
}

// findUpstream reads /etc/resolv.conf and returns the first nameserver address with port 53.
func findUpstream() string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Fatalf("cannot open /etc/resolv.conf: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			return net.JoinHostPort(fields[1], "53")
		}
	}
	log.Fatal("no nameserver found in /etc/resolv.conf")
	return ""
}

// lookupHostIP resolves a hostname via Go's built-in resolver (reads /etc/hosts)
// and returns the first IPv4 address.
func lookupHostIP(hostname string) net.IP {
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		log.Fatalf("cannot resolve %q: %v", hostname, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && ip.To4() != nil {
			return ip
		}
	}
	log.Fatalf("no IPv4 address found for %q", hostname)
	return nil
}
