package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mesa-black/stellar/web"
)

// serve runs the website until it is interrupted.
func serve(args []string) error {
	fs := flag.NewFlagSet("stellar serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen on `host:port`")
	base := fs.String("url", "", "public `address` of the site, e.g. https://stellar.example; shared messages link back to it")
	logs := fs.Bool("logs", false, "let visitors look a host up in the public certificate logs; its name is then sent to Cert Spotter (SSLMate)")
	fs.Parse(args)

	site := web.New(web.PublicLookup)
	if *logs {
		site.Logs = web.CertSpotter
	}
	if *base != "" {
		u, err := url.Parse(*base)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("-url %q is not an http(s) address", *base)
		}
		site.BaseURL = strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/")
		// The home page shows the site's own sky: fetch it before the first
		// visitor comes.
		go site.RefreshSelf()
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           site.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	fmt.Fprintf(os.Stderr, "stellar: listening on http://%s\n", *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
