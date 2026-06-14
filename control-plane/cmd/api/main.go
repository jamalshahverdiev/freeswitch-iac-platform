package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/api"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/audit"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/config"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/db"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/events"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/runtime"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	log.Info("migrations applied")

	st := store.New(pool)
	au := audit.New(pool)
	esl := runtime.New(cfg.ESLAddr, cfg.ESLPassword, cfg.ESLTimeout)

	// Optional read-only pool to freeswitch_core for the voicemail read API.
	var vmStore *store.VoicemailStore
	if cfg.CoreDatabaseURL != "" {
		corePool, err := db.Connect(ctx, cfg.CoreDatabaseURL)
		if err != nil {
			log.Error("core database connect", "err", err)
			os.Exit(1)
		}
		defer corePool.Close()
		vmStore = store.NewVoicemail(corePool)
		log.Info("voicemail read API enabled (freeswitch_core connected)")
	}

	// Live event stream: a persistent ESL listener feeds an in-process hub that
	// GET /api/v1/events (SSE) fans out. Runs only when ESL is configured.
	hub := events.NewHub()
	listenerCtx, stopListener := context.WithCancel(ctx)
	defer stopListener()
	if cfg.ESLAddr != "" {
		go events.NewListener(cfg.ESLAddr, cfg.ESLPassword, hub, log).Run(listenerCtx)
	}

	tlsEnabled := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	mtls := tlsEnabled && cfg.XMLClientCAFile != ""

	srv := api.NewServer(st, au, esl, api.Options{
		Token:                cfg.APIToken,
		XMLUser:              cfg.XMLUser,
		XMLPassword:          cfg.XMLPassword,
		XMLAllowCIDRs:        cfg.XMLAllowCIDRs,
		XMLRequireClientCert: mtls,
		CCOdbcDSN:            cfg.CCOdbcDSN,
		VMOdbcDSN:            cfg.VMOdbcDSN,
		VoicemailStore:       vmStore,
		RecURL:               cfg.RecURL,
		RecUser:              cfg.RecUser,
		RecPassword:          cfg.RecPassword,
		Hub:                  hub,
		ProvisionUser:        cfg.ProvisionUser,
		ProvisionPassword:    cfg.ProvisionPassword,
		ProvisionAllowCIDRs:  cfg.ProvisionAllowCIDRs,
		ProvisionSIPServer:   cfg.ProvisionSIPServer,
		ProvisionSIPPort:     cfg.ProvisionSIPPort,
	}, log)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if mtls {
		caPEM, err := os.ReadFile(cfg.XMLClientCAFile)
		if err != nil {
			log.Error("read XML_CLIENT_CA_FILE", "err", err)
			os.Exit(1)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			log.Error("XML_CLIENT_CA_FILE has no valid certificates")
			os.Exit(1)
		}
		// Verify a client cert if one is presented; xmlGuard enforces presence
		// on /xml/* only, so /api/v1 still works with just the bearer token.
		httpServer.TLSConfig = &tls.Config{
			ClientCAs:  pool,
			ClientAuth: tls.VerifyClientCertIfGiven,
			MinVersion: tls.VersionTLS12,
		}
	}

	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr, "tls", tlsEnabled, "mtls", mtls)
		var err error
		if tlsEnabled {
			err = httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("shutting down")
	stopListener() // stop the ESL event listener
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}
