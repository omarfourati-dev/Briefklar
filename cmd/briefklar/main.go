// Briefklar explains German authority letters. See README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/config"
	"github.com/omarfourati-dev/briefklar/internal/explain"
	"github.com/omarfourati-dev/briefklar/internal/extract"
	"github.com/omarfourati-dev/briefklar/internal/letters"
	"github.com/omarfourati-dev/briefklar/internal/metrics"
	"github.com/omarfourati-dev/briefklar/internal/server"
	"github.com/omarfourati-dev/briefklar/internal/store"
	"github.com/omarfourati-dev/briefklar/internal/users"
	"github.com/omarfourati-dev/briefklar/static"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check /healthz and exit (for Docker HEALTHCHECK)")
	flag.Parse()
	if *healthcheck {
		os.Exit(checkHealth())
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("stopped", "error", err)
		os.Exit(1)
	}
}

func checkHealth() int {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := auth.Bootstrap(ctx, st, cfg.AdminEmail, cfg.AdminPassword, cfg.DemoEnabled, cfg.DailyLimit); err != nil {
		return err
	}

	tokens, err := auth.NewTokens([]byte(cfg.JWTSecret), 8*time.Hour)
	if err != nil {
		return err
	}
	m := metrics.New()
	authSvc := auth.NewService(st, tokens)
	authSvc.Log = log
	authSvc.OnLogin = func(outcome string) { m.Logins.WithLabelValues(outcome).Inc() }

	var explainer explain.Explainer = explain.Fake{}
	if cfg.Explainer == "openai" {
		explainer = &explain.OpenAI{HTTP: &http.Client{}, BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, Log: log}
	}
	log.Info("starting", "addr", cfg.Addr, "explainer", cfg.Explainer, "demo", cfg.DemoEnabled)

	files, err := fs.Sub(static.Files, "dist")
	if err != nil {
		return err
	}
	handler := server.New(server.Deps{
		Store:   st,
		Auth:    authSvc,
		Letters: &letters.Handler{Extractor: extract.Default(), Explainer: explainer, Quota: st, Metrics: m, Now: time.Now, Log: log},
		Users:   &users.Handler{Store: st, Now: time.Now, DefaultLimit: cfg.DailyLimit},
		Metrics: m,
		Static:  files,
		Log:     log,
	})
	srv := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 2 * time.Minute, WriteTimeout: 3 * time.Minute, IdleTimeout: 2 * time.Minute}

	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		log.Warn("shutdown timed out, closing remaining connections")
		_ = srv.Close()
		return nil
	case err != nil && !errors.Is(err, http.ErrServerClosed):
		return err
	}
	return nil
}
