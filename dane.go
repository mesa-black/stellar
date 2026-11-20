package stellar

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// A site's owner can say, in the DNS, which key to expect from it: a TLSA
// record (DANE, RFC 6698) at _443._tcp.<host>. The form everybody is advised
// to publish, "3 1 1", carries the SHA-256 of the key's SubjectPublicKeyInfo:
// exactly the digest a sky is drawn from. So "the sky the owner announces" is
// not a format of ours; it is a record that already exists, read as a sky.
//
// Go's resolver does not ask for TLSA records, so the few lines of DNS it
// takes are written here: one question, over UDP, then TCP if the answer was
// cut short.
//
// What an answer is worth: this program does not validate DNSSEC. A record
// says what the DNS said to the resolver this machine uses, no more. It is a
// second road, independent of the certificate authorities — which is the
// point — but it is as trustworthy as that road.

// Announcement is what the DNS says about the key a host should present.
type Announcement struct {
	// Keys are the SHA-256 digests of the keys the owner announces, from the
	// records Stellar reads: certificate usage 1 or 3 (the server's own
	// certificate), selector 1 (its key), matching type 1 (SHA-256).
	Keys [][32]byte
	// Other counts the TLSA records of any other form: published, not read.
	Other int
}

// Announces reports whether a digest is among the announced keys.
func (a Announcement) Announces(digest [32]byte) bool {
	for _, k := range a.Keys {
		if k == digest {
			return true
		}
	}
	return false
}

// Record returns the TLSA record that announces a key, in zone-file form, for
// an owner to publish.
func Record(host string, digest [32]byte) string {
	return fmt.Sprintf("_443._tcp.%s. IN TLSA 3 1 1 %x", strings.TrimSuffix(host, "."), digest)
}

const typeTLSA = 52

// LookupAnnouncement asks the system's resolver for the TLSA records of a
// host's port 443. A host that publishes none gives an empty Announcement
// and no error.
func LookupAnnouncement(ctx context.Context, host string) (Announcement, error) {
	servers := resolvers()
	if len(servers) == 0 {
		return Announcement{}, errors.New("no DNS resolver configured")
	}
	query, id, err := dnsQuery("_443._tcp."+strings.TrimSuffix(host, "."), typeTLSA)
	if err != nil {
		return Announcement{}, err
	}
	var last error
	for _, server := range servers {
		answer, err := dnsExchange(ctx, server, query)
		if err != nil {
			last = err
			continue
		}
		records, err := dnsAnswers(answer, id, typeTLSA)
		if err != nil {
			last = err
			continue
		}
		var out Announcement
		for _, r := range records {
			// usage, selector, matching type, then the data.
			if len(r) == 3+32 && (r[0] == 1 || r[0] == 3) && r[1] == 1 && r[2] == 1 {
				out.Keys = append(out.Keys, [32]byte(r[3:]))
			} else {
				out.Other++
			}
		}
		return out, nil
	}
	return Announcement{}, last
}

// resolvers reads the name servers of /etc/resolv.conf, the file Go's own
// resolver reads on the systems this program runs on.
func resolvers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var out []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "nameserver" {
			out = append(out, net.JoinHostPort(fields[1], "53"))
		}
	}
	return out
}

// dnsQuery builds a query for one name and one type, recursion desired, with
// an EDNS0 record that lets the answer exceed 512 bytes over UDP.
func dnsQuery(name string, qtype uint16) (msg []byte, id uint16, err error) {
	var raw [2]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, 0, err
	}
	id = binary.BigEndian.Uint16(raw[:])
	msg = binary.BigEndian.AppendUint16(msg, id)
	msg = binary.BigEndian.AppendUint16(msg, 0x0100) // recursion desired
	msg = append(msg, 0, 1, 0, 0, 0, 0, 0, 1)        // one question, one additional
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, 0, fmt.Errorf("bad DNS name %q", name)
		}
		msg = append(msg, byte(len(label)))
		msg = append(msg, label...)
	}
	msg = append(msg, 0)
	msg = binary.BigEndian.AppendUint16(msg, qtype)
	msg = binary.BigEndian.AppendUint16(msg, 1) // class IN
	// OPT: root name, type 41, UDP payload 1232, no flags, no options.
	msg = append(msg, 0, 0, 41, 0x04, 0xd0, 0, 0, 0, 0, 0, 0)
	return msg, id, nil
}

// dnsExchange sends a query to one server over UDP, and again over TCP when
// the answer came back truncated.
func dnsExchange(ctx context.Context, server string, query []byte) ([]byte, error) {
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n >= 4 && buf[2]&0x02 == 0 { // not truncated
		return buf[:n], nil
	}

	tcp, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, err
	}
	defer tcp.Close()
	tcp.SetDeadline(deadline)
	framed := binary.BigEndian.AppendUint16(nil, uint16(len(query)))
	if _, err := tcp.Write(append(framed, query...)); err != nil {
		return nil, err
	}
	var size [2]byte
	if _, err := io.ReadFull(tcp, size[:]); err != nil {
		return nil, err
	}
	answer := make([]byte, binary.BigEndian.Uint16(size[:]))
	if _, err := io.ReadFull(tcp, answer); err != nil {
		return nil, err
	}
	return answer, nil
}

var errDNS = errors.New("malformed DNS answer")

// dnsAnswers returns the data of every answer record of one type. A name
// that does not exist, or exists without that type, gives none.
func dnsAnswers(msg []byte, id uint16, qtype uint16) ([][]byte, error) {
	if len(msg) < 12 || binary.BigEndian.Uint16(msg) != id || msg[2]&0x80 == 0 {
		return nil, errDNS
	}
	switch rcode := msg[3] & 0x0f; rcode {
	case 0, 3: // no error, or no such name
	default:
		return nil, fmt.Errorf("DNS answer code %d", rcode)
	}
	questions := int(binary.BigEndian.Uint16(msg[4:]))
	answers := int(binary.BigEndian.Uint16(msg[6:]))
	pos := 12
	var err error
	for i := 0; i < questions; i++ {
		if pos, err = dnsSkipName(msg, pos); err != nil {
			return nil, err
		}
		pos += 4
	}
	var out [][]byte
	for i := 0; i < answers; i++ {
		if pos, err = dnsSkipName(msg, pos); err != nil {
			return nil, err
		}
		if pos+10 > len(msg) {
			return nil, errDNS
		}
		rtype := binary.BigEndian.Uint16(msg[pos:])
		length := int(binary.BigEndian.Uint16(msg[pos+8:]))
		pos += 10
		if pos+length > len(msg) {
			return nil, errDNS
		}
		if rtype == qtype {
			out = append(out, msg[pos:pos+length])
		}
		pos += length
	}
	return out, nil
}

// dnsSkipName steps over a name, which ends with a zero byte or with a
// two-byte pointer to an earlier one.
func dnsSkipName(msg []byte, pos int) (int, error) {
	for {
		if pos >= len(msg) {
			return 0, errDNS
		}
		length := int(msg[pos])
		switch {
		case length == 0:
			return pos + 1, nil
		case length&0xc0 == 0xc0:
			return pos + 2, nil
		case length > 63:
			return 0, errDNS
		}
		pos += 1 + length
	}
}
