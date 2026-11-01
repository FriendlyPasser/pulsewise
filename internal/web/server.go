// Package web runs the panel in a browser without loading a native UI host.
package web

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

type host struct{ quit context.CancelFunc }

func (host) FitPanel(int) {} // A browser has no native panel to resize.
func (h host) Quit()      { h.quit() }

// Serve runs until the panel requests a quit or the server fails.
func Serve(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampler := stats.NewSampler(time.Second)
	go sampler.Run(ctx)
	srv := &http.Server{Handler: panel.Handler(host{quit: cancel}, sampler, nil)}
	shutdownDone := make(chan struct{})
	defer func() {
		cancel()
		<-shutdownDone
	}()
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		// Let the quit response finish before closing active connections.
		shutdownCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
		}
	}()
	log.Printf("pulsewise: serving the panel at http://%s", listener.Addr())
	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
