// Command stellar draws the constellation of a public key.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mesa-black/stellar"
)

const usage = `Usage:
  stellar [flags] <file>           an OpenSSH public key, PEM public key or PEM certificate
  stellar [flags] -                the same, read from standard input
  stellar [flags] SHA256:<base64>  a fingerprint, as printed by ssh-keygen -l
  stellar [flags] -host <name>     the certificate a TLS server presents
  stellar serve [-addr host:port]  the website
  stellar version                  the version of this program

Flags:
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stellar:", err)
		os.Exit(1)
	}
}

// version is set at build time (-ldflags "-X main.version=…").
var version = "dev"

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		return serve(os.Args[2:])
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("stellar %s (mapping v%d)\n", version, stellar.Version)
		return nil
	}
	host := flag.String("host", "", "fetch the key from a TLS server, `name[:port]`")
	insecure := flag.Bool("insecure", false, "with -host, do not verify the certificate chain")
	out := flag.String("o", "", "write an SVG to `file` (- for standard output)")
	size := flag.Int("size", 640, "SVG width and height, in pixels")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	key, seen, err := load(*host, *insecure, flag.Args())
	if err != nil {
		return err
	}
	c := stellar.New(key.Digest)

	if *out == "-" {
		_, err := io.WriteString(os.Stdout, c.SVG(*size, key.Fingerprint()))
		return err
	}
	if *out != "" {
		if err := os.WriteFile(*out, []byte(c.SVG(*size, key.Fingerprint())), 0o644); err != nil {
			return err
		}
	} else {
		fmt.Print(c.Text(useColor()))
	}
	fmt.Printf("Name:        %s\n", c.Name)
	fmt.Printf("Fingerprint: %s\n", key.Fingerprint())
	if key.Label != "" {
		fmt.Printf("Key:         %s (%s)\n", key.Label, key.Kind)
	} else {
		fmt.Printf("Key:         %s\n", key.Kind)
	}
	if seen != nil {
		// What the certificate and the handshake say, beside the key.
		fmt.Printf("Certificate: %s, until %s, %s\n", seen.KeyAlgorithm, seen.NotAfter.UTC().Format("2006-01-02"), seen.Issuer)
		exchange := seen.KeyExchange
		switch {
		case exchange == "":
			exchange = "RSA key exchange"
		case seen.PostQuantum:
			exchange += " (post-quantum hybrid)"
		default:
			exchange += " (not post-quantum)"
		}
		fmt.Printf("Connection:  %s, %s\n", seen.Version, exchange)
	}
	if *out != "" {
		fmt.Printf("Written to:  %s\n", *out)
	}
	return nil
}

// load returns the key to draw and, for a TLS server, what it showed of
// itself beside it.
func load(host string, insecure bool, args []string) (stellar.Key, *stellar.Host, error) {
	if host != "" {
		if len(args) != 0 {
			return stellar.Key{}, nil, fmt.Errorf("-host takes no other argument")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		seen, err := stellar.InspectHost(ctx, nil, host, insecure)
		if err != nil {
			return stellar.Key{}, nil, err
		}
		return seen.Key, &seen, nil
	}
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	arg := args[0]
	if arg == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return stellar.Key{}, nil, err
		}
		key, err := stellar.Parse(data)
		return key, nil, err
	}
	data, err := os.ReadFile(arg)
	if os.IsNotExist(err) {
		// Not a file: accept a fingerprint given on the command line.
		if key, ferr := stellar.ParseFingerprint(arg); ferr == nil {
			return key, nil, nil
		}
	}
	if err != nil {
		return stellar.Key{}, nil, err
	}
	key, err := stellar.Parse(data)
	return key, nil, err
}

// useColor reports whether standard output is a terminal that wants colour.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
