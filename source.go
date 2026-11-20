package stellar

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Key identifies a public key by its SHA-256 digest.
type Key struct {
	Digest [32]byte
	// Kind describes where the digest comes from, e.g. "ssh-ed25519" or
	// "X.509 certificate".
	Kind string
	// Label is a human hint about the key: an SSH comment, a certificate
	// subject, a host name. It is not covered by the digest.
	Label string
}

// Fingerprint returns the digest in the OpenSSH notation: "SHA256:" followed
// by unpadded base64.
func (k Key) Fingerprint() string {
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(k.Digest[:])
}

// Parse reads a public key from data, which may hold an OpenSSH public key
// line, a PEM public key, a PEM certificate, or a "SHA256:..." fingerprint.
func Parse(data []byte) (Key, error) {
	trimmed := bytes.TrimSpace(data)
	switch {
	case bytes.HasPrefix(trimmed, []byte("SHA256:")):
		return ParseFingerprint(string(trimmed))
	case bytes.Contains(trimmed, []byte("-----BEGIN ")):
		return ParsePEM(trimmed)
	default:
		return ParseSSH(trimmed)
	}
}

// ParseFingerprint reads a "SHA256:..." fingerprint as printed by
// ssh-keygen -l.
func ParseFingerprint(s string) (Key, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(strings.TrimPrefix(s, "SHA256:"), "="))
	if err != nil || len(raw) != sha256.Size {
		return Key{}, errors.New("not a SHA256 fingerprint: want SHA256: followed by 43 base64 characters")
	}
	return Key{Digest: [32]byte(raw), Kind: "fingerprint"}, nil
}

// ParseSSH reads the first key of an OpenSSH public key file
// (authorized_keys format). The digest is the one ssh-keygen -l prints.
func ParseSSH(data []byte) (Key, error) {
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		// The key type may be preceded by options; find the field whose
		// successor is a blob announcing that same type.
		fields := strings.Fields(l)
		for i := 0; i+1 < len(fields); i++ {
			blob, err := base64.StdEncoding.DecodeString(fields[i+1])
			if err != nil || !blobHasType(blob, fields[i]) {
				continue
			}
			return Key{
				Digest: sha256.Sum256(blob),
				Kind:   fields[i],
				Label:  strings.Join(fields[i+2:], " "),
			}, nil
		}
	}
	return Key{}, errors.New("no OpenSSH public key found")
}

// blobHasType reports whether an SSH wire-format key blob starts with the
// given key type, as a length-prefixed string.
func blobHasType(blob []byte, keyType string) bool {
	if len(blob) < 4 {
		return false
	}
	n := binary.BigEndian.Uint32(blob)
	return uint64(n) == uint64(len(keyType)) && len(blob) >= 4+len(keyType) && string(blob[4:4+n]) == keyType
}

// ParsePEM reads a PEM public key or certificate. The digest covers the
// DER-encoded SubjectPublicKeyInfo, so a certificate and the bare public key
// it certifies yield the same constellation, and a renewed certificate keeps
// its constellation as long as it keeps its key.
func ParsePEM(data []byte) (Key, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return Key{}, errors.New("no PEM public key or certificate found")
		}
		switch {
		case block.Type == "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return Key{}, fmt.Errorf("parsing certificate: %w", err)
			}
			return certKey(cert), nil
		case block.Type == "PUBLIC KEY":
			if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
				return Key{}, fmt.Errorf("parsing public key: %w", err)
			}
			return Key{Digest: sha256.Sum256(block.Bytes), Kind: "public key"}, nil
		case strings.Contains(block.Type, "PRIVATE KEY"):
			return Key{}, errors.New("this is a private key: stellar only reads public keys")
		}
	}
}

// FromHost connects to a TLS server and returns the key of the certificate it
// presents. addr is a host name, with an optional port (443 by default).
// With insecure set, the certificate chain is not verified: the key is then
// whatever the network path chose to present.
func FromHost(ctx context.Context, addr string, insecure bool) (Key, error) {
	return FromHostWith(ctx, nil, addr, insecure)
}

// FromHostWith is FromHost over a caller-supplied dialer, which lets a
// server restrict the addresses it agrees to connect to. A nil dialer means
// the default one.
func FromHostWith(ctx context.Context, netDialer *net.Dialer, addr string, insecure bool) (Key, error) {
	host, err := InspectHost(ctx, netDialer, addr, insecure)
	return host.Key, err
}

// Host is what a TLS server showed of itself during one handshake: its key,
// and how the connection that carried it was protected.
type Host struct {
	Key Key
	// Version is the protocol version negotiated, e.g. "TLS 1.3".
	Version string
	// KeyExchange is the group the two sides agreed on to derive the session
	// keys, e.g. "X25519MLKEM768". Empty for a legacy RSA key exchange.
	KeyExchange string
	// PostQuantum reports whether that group includes a post-quantum KEM.
	// It is about the secrecy of the session, not about the key drawn: the
	// server's own key is a classical one in every certificate issued today.
	PostQuantum bool
	// KeyAlgorithm describes the certificate's key, e.g. "ECDSA P-256".
	KeyAlgorithm string
	// Issuer names who signed the certificate.
	Issuer string
	// NotAfter is when the certificate expires.
	NotAfter time.Time
	// Announcement is what the DNS says the host's key should be (see
	// LookupAnnouncement). InspectHost leaves it nil; nil means nobody asked,
	// or the question got no usable answer.
	Announcement *Announcement
}

// InspectHost connects to a TLS server like FromHostWith and also reports
// what was negotiated. The key exchange is the one agreed with this program,
// which offers the hybrid post-quantum group first: a server that supports
// it is seen to.
func InspectHost(ctx context.Context, netDialer *net.Dialer, addr string, insecure bool) (Host, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}
	dialer := tls.Dialer{NetDialer: netDialer, Config: &tls.Config{InsecureSkipVerify: insecure}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Host{}, err
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	cert := state.PeerCertificates[0]

	host := Host{
		Key:          certKey(cert),
		Version:      tls.VersionName(state.Version),
		KeyAlgorithm: keyAlgorithm(cert),
		Issuer:       issuer(cert),
		NotAfter:     cert.NotAfter,
	}
	host.Key.Label = addr
	if state.CurveID != 0 {
		host.KeyExchange = state.CurveID.String()
	}
	switch state.CurveID {
	case tls.X25519MLKEM768, tls.SecP256r1MLKEM768, tls.SecP384r1MLKEM1024, tls.MLKEM1024:
		host.PostQuantum = true
	}
	return host, nil
}

func keyAlgorithm(cert *x509.Certificate) string {
	switch pub := cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		return "ECDSA " + pub.Curve.Params().Name
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", pub.N.BitLen())
	case ed25519.PublicKey:
		return "Ed25519"
	}
	return cert.PublicKeyAlgorithm.String()
}

func issuer(cert *x509.Certificate) string {
	name := cert.Issuer.CommonName
	if orgs := cert.Issuer.Organization; len(orgs) > 0 && !strings.Contains(name, orgs[0]) {
		name = strings.TrimSpace(orgs[0] + " " + name)
	}
	return name
}

func certKey(cert *x509.Certificate) Key {
	return Key{
		Digest: sha256.Sum256(cert.RawSubjectPublicKeyInfo),
		Kind:   "X.509 certificate",
		Label:  cert.Subject.CommonName,
	}
}
