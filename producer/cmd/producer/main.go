package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(ctx, "received request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"message": "Hello, World!"}`))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.ErrorContext(ctx, "failed to write response", "error", err)
			return
		}
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "could not listen on port 8901", "error", err)
		}
	}()

	<-ctx.Done()

	slog.InfoContext(ctx, "shutting down gracefully, press Ctrl+C again to force")

	if err := server.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server forced to shutdown", "error", err)
	}
}
