// Package routes wires middleware and handlers into a Gin engine.
package routes

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhmaitran/compatlab/apps/api/handlers"
	"github.com/thanhmaitran/compatlab/apps/api/middleware"
	"github.com/thanhmaitran/compatlab/internal/service"
)

// Config tunes the HTTP layer.
type Config struct {
	MaxBodyBytes   int64         // default 5 MiB
	RateLimit      int           // requests per window per IP for the API; 0 disables
	AuthRateLimit  int           // requests per window per IP for register/login; 0 disables
	RateWindow     time.Duration // default 1 minute
	CORSOrigin     string        // single allowed browser origin; empty disables CORS
	TrustedProxies []string      // proxies whose X-Forwarded-For is trusted; empty trusts none
}

// New builds the router.
func New(cfg Config, svc *service.Service) *gin.Engine {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 5 << 20
	}
	if cfg.RateWindow <= 0 {
		cfg.RateWindow = time.Minute
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.Use(gin.Recovery(), gin.Logger(), middleware.SecurityHeaders(), middleware.CORS(cfg.CORSOrigin))

	h := &handlers.Handlers{Svc: svc}
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)

	v1 := r.Group("/api/v1", middleware.RequestSize(cfg.MaxBodyBytes), middleware.RateLimit(cfg.RateLimit, cfg.RateWindow))
	authRL := middleware.RateLimit(cfg.AuthRateLimit, cfg.RateWindow)
	v1.POST("/auth/register", authRL, h.Register)
	v1.POST("/auth/login", authRL, h.Login)

	api := v1.Group("", middleware.Auth(svc.Tokens))
	api.POST("/users", h.CreateUser)

	api.POST("/projects", h.CreateProject)
	api.GET("/projects", h.ListProjects)
	api.GET("/projects/:id", h.GetProject)
	api.POST("/projects/:id/specifications", h.UploadSpecification)
	api.GET("/projects/:id/versions", h.ListVersions)
	api.POST("/projects/:id/analyze", h.Analyze)
	api.GET("/projects/:id/analyses", h.ListAnalyses)
	api.GET("/projects/:id/contracts", h.ListContracts)
	api.GET("/projects/:id/policies", h.ListPolicies)
	api.GET("/projects/:id/test-runs", h.ListRuns)
	api.GET("/projects/:id/gates", h.ListGates)
	api.GET("/projects/:id/analytics", h.Analytics)

	api.GET("/versions/:id/specification", h.InspectVersion)
	api.GET("/analyses/:id", h.GetAnalysis)
	api.GET("/analyses/:id/changes", h.AnalysisChanges)

	api.POST("/contracts", h.RegisterContract)
	api.POST("/contracts/:id/verify", h.VerifyContract)

	api.POST("/test-suites/generate", h.GenerateSuite)
	api.GET("/test-suites/:id", h.GetSuite)
	api.POST("/test-runs", h.RunTests)
	api.GET("/test-runs/:id", h.GetRun)

	api.POST("/policies", h.CreatePolicy)
	api.POST("/gate/evaluate", h.EvaluateGate)
	return r
}
