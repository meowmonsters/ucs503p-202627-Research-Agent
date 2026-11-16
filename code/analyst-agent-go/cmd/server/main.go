// Command server is the ResearchAgent backend: it exposes the research
// agent and its tools over HTTP for the React frontend to call.
//
//	GET  /healthz                 liveness check
//	POST /api/agent               {"company": "..."} -> {"messages": [...]}
//	POST /api/tools/search        {"query": "..."}
//	POST /api/tools/financials    {"ticker": "..."}
//	POST /api/tools/news          {"company": "..."}
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"analystagent/internal/httpapi"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", httpapi.HandleHealth)
	mux.HandleFunc("/api/agent", httpapi.HandleAgent)
	mux.HandleFunc("/api/tools/search", httpapi.HandleToolSearch)
	mux.HandleFunc("/api/tools/financials", httpapi.HandleToolFinancials)
	mux.HandleFunc("/api/tools/news", httpapi.HandleToolNews)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Printf("warning: GEMINI_API_KEY is not set — the agent will use its deterministic fallback instead of the AI model")
	}
	if os.Getenv("ALLOWED_ORIGIN") == "" {
		log.Printf("note: ALLOWED_ORIGIN is not set — any website may call this API. Set it to your frontend's URL in production.")
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.CORS(httpapi.Recover(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// An agent run makes several LLM and network calls in one request,
		// so the write timeout has to comfortably exceed a full run (the
		// agent's own time budget is 90s by default, plus the final call).
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Serve in the background so SIGTERM (what Render sends on every
	// deploy and restart) can drain in-flight research runs instead of
	// cutting them off mid-request.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("ResearchAgent backend listening on %s", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		// Failed to start (e.g. port already in use).
		log.Fatal(err)
	case sig := <-stop:
		log.Printf("received %s — shutting down, waiting for in-flight requests", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("shutdown did not finish cleanly: %v", err)
	}
	log.Printf("bye")
}
