package stellar

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestInspectHost reads a real handshake, against a local server: the key is
// the certificate's, and a Go server and a Go client agree on the hybrid
// post-quantum group.
func TestInspectHost(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	host, err := InspectHost(ctx, nil, srv.Listener.Addr().String(), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := sha256.Sum256(srv.Certificate().RawSubjectPublicKeyInfo); host.Key.Digest != want {
		t.Error("the key is not the certificate's")
	}
	if host.Version != "TLS 1.3" {
		t.Errorf("version = %q", host.Version)
	}
	if host.KeyExchange != "X25519MLKEM768" || !host.PostQuantum {
		t.Errorf("key exchange = %q, post-quantum = %v", host.KeyExchange, host.PostQuantum)
	}
	if host.KeyAlgorithm == "" || host.Issuer == "" || host.NotAfter.IsZero() {
		t.Errorf("certificate = %q, issuer %q, until %v", host.KeyAlgorithm, host.Issuer, host.NotAfter)
	}

	key, err := FromHostWith(ctx, nil, srv.Listener.Addr().String(), true)
	if err != nil || key.Digest != host.Key.Digest {
		t.Errorf("FromHostWith disagrees with InspectHost: %v", err)
	}
	if _, err := InspectHost(ctx, nil, srv.Listener.Addr().String(), false); err == nil {
		t.Error("a self-signed certificate passed verification")
	}
}
