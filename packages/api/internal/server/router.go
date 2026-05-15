package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/uddi-protocol/uddi/api/internal/apidocs"
	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/config"
	"github.com/uddi-protocol/uddi/api/internal/handlers"
	"github.com/uddi-protocol/uddi/api/internal/middleware"
	"github.com/uddi-protocol/uddi/api/internal/zkp"
)

func NewRouter(cfg *config.Config, chainClient *blockchain.Client, zkpService *zkp.Service) (http.Handler, error) {
	didHandler := handlers.NewDIDHandler(chainClient)
	challengeStore, err := newChallengeStore(cfg)
	if err != nil {
		return nil, err
	}
	apiKeyStore, err := newAPIKeyStore(cfg)
	if err != nil {
		return nil, err
	}
	credentialStore, err := newCredentialStore(cfg)
	if err != nil {
		return nil, err
	}
	verifyHandler := handlers.NewVerifyHandlerWithChallengeStore(chainClient, zkpService, challengeStore)
	credHandler := handlers.NewCredentialHandler(chainClient, credentialStore)
	apiKeyHandler := handlers.NewAPIKeyHandler(apiKeyStore)
	proofHandler := handlers.NewProofHandler(zkpService, chainClient)
	registryHandler := handlers.NewRegistryHandler(chainClient)

	r := chi.NewRouter()
	metrics := middleware.NewMetrics()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(metrics.Middleware)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.LimitRequestBody(cfg.MaxRequestBodyBytes))
	r.Use(middleware.NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow).Middleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-Service-ID", "X-Admin-Token"},
		AllowCredentials: true,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"0.1.0"}`))
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready","version":"0.1.0"}`))
	})
	r.Get("/metrics", metrics.Handler)
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(apidocs.OpenAPIYAML)
	})
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Route("/did", func(r chi.Router) {
			r.Post("/register", didHandler.Register)
			r.Get("/{did}", didHandler.Resolve)
			r.Post("/revoke", didHandler.Revoke)
			r.Put("/{did}/update", didHandler.Update)
		})

		r.Route("/credentials", func(r chi.Router) {
			r.Use(middleware.RequireAPIKey(apiKeyStore))
			r.Get("/{did}", credHandler.ListByDID)
			r.Post("/issue", credHandler.Issue)
			r.Post("/revoke", credHandler.Revoke)
			r.Get("/{id}/verify", credHandler.Verify)
		})

		r.Route("/verify", func(r chi.Router) {
			r.Use(middleware.RequireAPIKey(apiKeyStore))
			r.Post("/challenge", verifyHandler.CreateChallenge)
			r.Post("/auth", verifyHandler.VerifyAuth)
			r.Post("/claim", verifyHandler.VerifyClaim)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.RequireAdminToken(cfg.AdminToken))
			r.Route("/api-keys", func(r chi.Router) {
				r.Get("/", apiKeyHandler.List)
				r.Post("/", apiKeyHandler.Create)
				r.Post("/revoke", apiKeyHandler.Revoke)
			})
		})

		r.Route("/proof", func(r chi.Router) {
			r.Post("/generate", proofHandler.Generate)
		})

		r.Get("/registry/stats", registryHandler.Stats)
	})

	return r, nil
}

func newAPIKeyStore(cfg *config.Config) (middleware.APIKeyStore, error) {
	if cfg.DatabaseURL == "" {
		return middleware.NewMemoryAPIKeyStore(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := middleware.NewPostgresAPIKeyStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create Postgres API key store: %w", err)
	}
	return store, nil
}

const swaggerUIHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>UDDI API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
      body { margin: 0; background: #ffffff; }
      .swagger-ui .topbar { display: none; }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        persistAuthorization: true,
        displayRequestDuration: true
      });
    </script>
  </body>
</html>`

func newChallengeStore(cfg *config.Config) (handlers.ChallengeStore, error) {
	if cfg.RedisURL == "" {
		return handlers.NewMemoryChallengeStore(), nil
	}

	store, err := handlers.NewRedisChallengeStore(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("create Redis challenge store: %w", err)
	}
	return store, nil
}

func newCredentialStore(cfg *config.Config) (handlers.CredentialStore, error) {
	if cfg.DatabaseURL == "" {
		return handlers.NewMemoryCredentialStore(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := handlers.NewPostgresCredentialStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create Postgres credential store: %w", err)
	}
	return store, nil
}
