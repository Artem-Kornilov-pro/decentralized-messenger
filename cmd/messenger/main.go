// Command messenger runs the decentralized messenger node. With -demo it runs
// an in-process demonstration of signing, chaining, snapshotting, and
// verification; otherwise it starts the HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/api"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/cache"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

// Server tuning. These are conservative defaults that defend against slow
// clients (e.g. slowloris) while leaving room for 10 MiB photo uploads.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second
	// maxRequestBytes caps a request body. A 50 MiB video (the largest
	// attachment) inflates ~1.37x under base64 and travels with JSON envelope
	// fields, so allow generous headroom.
	maxRequestBytes = 72 << 20
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	demo := flag.Bool("demo", false, "run an in-process demonstration and exit")
	flag.Parse()

	lg := chatlog.New(buildStorage(), chatlog.WithCache(buildCache()), chatlog.WithBroker(buildBroker()))
	svc := service.New(lg)

	if *demo {
		if err := runDemo(svc); err != nil {
			log.Fatalf("demo failed: %v", err)
		}
		return
	}

	if err := serve(*addr, api.NewServer(svc).Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// serve runs the HTTP server with hardened timeouts and shuts it down
// gracefully on SIGINT/SIGTERM, draining in-flight requests.
func serve(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.MaxBytesHandler(handler, maxRequestBytes),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Translate termination signals into a context cancellation.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runUntilShutdown(ctx, srv)
}

// runUntilShutdown serves until srv fails or ctx is cancelled, then drains
// in-flight requests within shutdownTimeout. It is decoupled from OS signals so
// the shutdown path can be tested with any cancellable context.
func runUntilShutdown(ctx context.Context, srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		log.Printf("messenger node listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Print("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		log.Print("shutdown complete")
		return nil
	}
}

// buildStorage selects the ScyllaDB adapter when SCYLLA_HOSTS is set (comma
// separated), falling back to the in-memory store otherwise.
func buildStorage() storage.Storage {
	hosts := os.Getenv("SCYLLA_HOSTS")
	if hosts == "" {
		log.Print("storage: using in-memory store")
		return storage.NewInMemoryStorage()
	}
	keyspace := envOr("SCYLLA_KEYSPACE", "messenger")
	s, err := storage.NewScylla(keyspace, strings.Split(hosts, ",")...)
	if err != nil {
		log.Fatalf("storage: connect ScyllaDB: %v", err)
	}
	log.Printf("storage: using ScyllaDB keyspace %q at %s", keyspace, hosts)
	return s
}

// buildCache selects the Redis adapter when REDIS_ADDR is set.
func buildCache() cache.Cache {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Print("cache: using in-memory cache")
		return cache.NewInMemory()
	}
	log.Printf("cache: using Redis at %s", addr)
	return cache.NewRedis(addr, os.Getenv("REDIS_PASSWORD"), 0, time.Hour)
}

// buildBroker selects the RabbitMQ adapter when RABBITMQ_URL is set.
func buildBroker() broker.Broker {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		log.Print("broker: using in-memory broker")
		return broker.NewInMemory()
	}
	b, err := broker.NewRabbitMQ(url)
	if err != nil {
		log.Fatalf("broker: connect RabbitMQ: %v", err)
	}
	log.Printf("broker: using RabbitMQ at %s", url)
	return b
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func runDemo(svc *service.Messenger) error {
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		return err
	}
	const chatID = "demo-chat"

	for i := 1; i <= 3; i++ {
		// A real client builds and signs the message locally, then submits
		// only the result; the private key never leaves this point.
		msg := models.NewMessage(chatID, "alice", pub, []byte(fmt.Sprintf("message %d", i)), models.ContentTypeText, "", false)
		msg = crypto.SignMessage(msg, priv)

		entry, err := svc.Submit(msg, 0)
		if err != nil {
			return err
		}
		fmt.Printf("appended seq=%d hash=%s…\n", entry.Sequence, entry.EntryHash[:16])
	}

	result, err := svc.Verify(chatID)
	if err != nil {
		return err
	}
	fmt.Printf("verify: valid=%t entries=%d %s\n", result.Valid, result.Entries, result.Reason)

	bundle, err := svc.Sync(chatID)
	if err != nil {
		return err
	}
	fmt.Printf("sync: %d new entries, current hash %s…\n", len(bundle.NewEntries), bundle.CurrentHash[:16])

	if !result.Valid {
		os.Exit(1)
	}
	return nil
}
