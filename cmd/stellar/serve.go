package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mesa-black/stellar/web"
)

// serve runs the website until it is interrupted.
func serve(args []string) error {
	fs := flag.NewFlagSet("stellar serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen on `host:port`")
	fs.Parse(args)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           web.New(web.PublicLookup).Handler(),
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
