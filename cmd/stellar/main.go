// Command stellar draws the constellation of a public key.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/mesa-black/stellar"
)

const usage = `Usage:
  stellar [flags] <file>           an OpenSSH public key, PEM public key or PEM certificate
  stellar [flags] -                the same, read from standard input
  stellar [flags] SHA256:<base64>  a fingerprint, as printed by ssh-keygen -l
  stellar [flags] -host <name>     the certificate a TLS server presents
  stellar serve [-addr host:port]  the website
  stellar announce -zone <zone> <host>...
                                   publish the hosts' keys as DANE records (Gandi)
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
	if len(os.Args) > 1 && os.Args[1] == "announce" {
		return announce(os.Args[2:])
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("stellar %s (mapping v%d)\n", version, stellar.Version)
		return nil
	}
	host := flag.String("host", "", "fetch the key from a TLS server, `name[:port]`")
	insecure := flag.Bool("insecure", false, "with -host, do not verify the certificate chain")
	out := flag.String("o", "", "write an SVG to `file`, or a PNG if it ends in .png (- for SVG on standard output)")
	size := flag.Int("size", 640, "width and height of the picture, in pixels")
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
		// The extension chooses the format: a .png is a picture without
		// text, anything else the SVG with its name and fingerprint.
		data := []byte(c.SVG(*size, key.Fingerprint()))
		if strings.HasSuffix(strings.ToLower(*out), ".png") {
			data = c.PNG(*size, *size)
		}
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			return err
		}
	} else {
		fmt.Print(c.Text(useColor()))
	}
	fmt.Printf("Name:        %s\n", c.Name)
	fmt.Printf("Fingerprint: %s\n", key.Fingerprint())
	// The gate: the six signs of the key, by the first word of their names.
	var signs []string
	for _, k := range stellar.Gate(key.Digest) {
		word, _, _ := strings.Cut(stellar.Sign(k).Name, " ")
		signs = append(signs, word)
	}
	fmt.Printf("Gate:        %s\n", strings.Join(signs, " · "))
	// Where the gate leads, in the sphere of the signs: what the website
	// draws as the arrival on its chart.
	place := stellar.Place(key.Digest)
	fmt.Printf("Place:       %+.3f %+.3f %+.3f\n", place[0], place[1], place[2])
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
		// What the owner announces in the DNS, when the DNS answered.
		if a := seen.Announcement; a != nil {
			switch {
			case a.Announces(key.Digest):
				if a.Signed {
					fmt.Println("Announced:   yes, by a DANE record in the domain's DNS, signed (DNSSEC, checked by this machine's resolver)")
				} else {
					fmt.Println("Announced:   yes, by a DANE record in the domain's DNS (not signed, or this machine's resolver does not check DNSSEC)")
				}
			case len(a.Keys) > 0:
				names := make([]string, len(a.Keys))
				for i, k := range a.Keys {
					names[i] = stellar.New(k).Name
				}
				fmt.Printf("Announced:   ANOTHER KEY. The DNS announces %s\n", strings.Join(names, ", "))
			case a.Other > 0:
				fmt.Println("Announced:   DANE records of a form stellar does not read")
			default:
				fmt.Println("Announced:   no, the domain publishes no DANE record")
			}
		}
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
		name, _, splitErr := net.SplitHostPort(host)
		if splitErr != nil {
			name = host
		}
		if announced, err := stellar.LookupAnnouncement(ctx, name); err == nil {
			seen.Announcement = &announced
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
