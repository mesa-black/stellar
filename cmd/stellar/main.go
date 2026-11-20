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

Flags:
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stellar:", err)
		os.Exit(1)
	}
}

func run() error {
	host := flag.String("host", "", "fetch the key from a TLS server, `name[:port]`")
	insecure := flag.Bool("insecure", false, "with -host, do not verify the certificate chain")
	out := flag.String("o", "", "write an SVG to `file` (- for standard output)")
	size := flag.Int("size", 640, "SVG width and height, in pixels")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	key, err := load(*host, *insecure, flag.Args())
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
	if *out != "" {
		fmt.Printf("Written to:  %s\n", *out)
	}
	return nil
}

func load(host string, insecure bool, args []string) (stellar.Key, error) {
	if host != "" {
		if len(args) != 0 {
			return stellar.Key{}, fmt.Errorf("-host takes no other argument")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return stellar.FromHost(ctx, host, insecure)
	}
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	arg := args[0]
	if arg == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return stellar.Key{}, err
		}
		return stellar.Parse(data)
	}
	data, err := os.ReadFile(arg)
	if os.IsNotExist(err) {
		// Not a file: accept a fingerprint given on the command line.
		if key, ferr := stellar.ParseFingerprint(arg); ferr == nil {
			return key, nil
		}
	}
	if err != nil {
		return stellar.Key{}, err
	}
	return stellar.Parse(data)
}

// useColor reports whether standard output is a terminal that wants colour.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
