// Package handlers contains the thin HTTP layer: bind the request, call the
// service, map errors to status codes. No compatibility logic lives here.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhmaitran/compatlab/apps/api/dto"
	"github.com/thanhmaitran/compatlab/apps/api/middleware"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/service"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

// Handlers exposes the REST API.
type Handlers struct {
	Svc *service.Service
}

func actor(c *gin.Context) service.Actor {
	v, _ := c.Get(middleware.ActorKey)
	a, _ := v.(service.Actor)
	return a
}

func ctx(c *gin.Context) context.Context { return c.Request.Context() }

// status maps an error to an HTTP status and a client-safe message.
func status(err error) (int, string) {
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge, "request body too large"
	case errors.Is(err, service.ErrUnauthorized):
		return http.StatusUnauthorized, "invalid credentials"
	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, persistence.ErrNotFound):
		return http.StatusNotFound, "not found"
	case errors.Is(err, persistence.ErrConflict):
		return http.StatusConflict, "already exists"
	case errors.Is(err, domain.ErrInvalid),
		errors.Is(err, specification.ErrParse),
		errors.Is(err, specification.ErrValidation),
		errors.Is(err, specification.ErrUnsupported),
		errors.Is(err, specification.ErrTooLarge),
		errors.Is(err, specification.ErrReference),
		errors.Is(err, security.ErrTargetNotAllowed):
		return http.StatusBadRequest, err.Error()
	}
	log.Printf("internal error: %v", err)
	return http.StatusInternalServerError, "internal error"
}

func fail(c *gin.Context, err error) {
	code, msg := status(err)
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}

func bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(c, err)
		} else {
			fail(c, fmt.Errorf("%w: invalid request body: %v", domain.ErrInvalid, err))
		}
		return false
	}
	return true
}

// Health reports that the process is running.
func (h *Handlers) Health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }

// Ready reports whether dependencies (the database) are reachable.
func (h *Handlers) Ready(c *gin.Context) {
	pctx, cancel := context.WithTimeout(ctx(c), 2*time.Second)
	defer cancel()
	if err := h.Svc.Store.Ping(pctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *Handlers) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if !bind(c, &req) {
		return
	}
	token, err := h.Svc.Register(ctx(c), req.Organization, req.Email, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token})
}

func (h *Handlers) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bind(c, &req) {
		return
	}
	token, err := h.Svc.Login(ctx(c), req.Email, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handlers) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if !bind(c, &req) {
		return
	}
	u, err := h.Svc.CreateMember(ctx(c), actor(c), req.Email, req.Password, req.Role)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": u.ID, "email": u.Email, "role": u.Role})
}

func (h *Handlers) CreateProject(c *gin.Context) {
	var req dto.CreateProjectRequest
	if !bind(c, &req) {
		return
	}
	p, err := h.Svc.CreateProject(ctx(c), actor(c), req.Name, req.Description)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handlers) ListProjects(c *gin.Context) {
	list, err := h.Svc.ListProjects(ctx(c), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) GetProject(c *gin.Context) {
	p, err := h.Svc.GetProject(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handlers) UploadSpecification(c *gin.Context) {
	var req dto.UploadSpecRequest
	if !bind(c, &req) {
		return
	}
	res, err := h.Svc.UploadSpecification(ctx(c), actor(c), c.Param("id"), req.VersionLabel, req.Filename, []byte(req.Content))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handlers) ListVersions(c *gin.Context) {
	list, err := h.Svc.ListVersions(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) Analyze(c *gin.Context) {
	var req dto.AnalyzeRequest
	if !bind(c, &req) {
		return
	}
	v, err := h.Svc.Analyze(ctx(c), actor(c), c.Param("id"), req.BaseVersionID, req.TargetVersionID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) ListAnalyses(c *gin.Context) {
	list, err := h.Svc.ListAnalyses(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) GetAnalysis(c *gin.Context) {
	v, err := h.Svc.GetAnalysis(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handlers) AnalysisChanges(c *gin.Context) {
	list, err := h.Svc.AnalysisChanges(ctx(c), actor(c), c.Param("id"), c.Query("classification"), c.Query("direction"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) RegisterContract(c *gin.Context) {
	var req dto.RegisterContractRequest
	if !bind(c, &req) {
		return
	}
	v, err := h.Svc.RegisterContract(ctx(c), actor(c), req.ProjectID, req.Contract)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) ListContracts(c *gin.Context) {
	list, err := h.Svc.ListContracts(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) VerifyContract(c *gin.Context) {
	var req dto.VerifyContractRequest
	if !bind(c, &req) {
		return
	}
	res, err := h.Svc.VerifyContract(ctx(c), actor(c), c.Param("id"), req.VersionID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handlers) GenerateSuite(c *gin.Context) {
	var req dto.GenerateSuiteRequest
	if !bind(c, &req) {
		return
	}
	cfg := domain.GenerationConfig{Seed: req.Seed, MaxCasesPerEndpoint: req.MaxCasesPerEndpoint}
	v, err := h.Svc.GenerateSuite(ctx(c), actor(c), req.VersionID, req.AnalysisID, cfg)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) GetSuite(c *gin.Context) {
	v, err := h.Svc.GetSuite(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handlers) RunTests(c *gin.Context) {
	var req dto.RunTestsRequest
	if !bind(c, &req) {
		return
	}
	v, err := h.Svc.RunTests(ctx(c), actor(c), req.SuiteID, req.VersionID, req.Target)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) GetRun(c *gin.Context) {
	v, err := h.Svc.GetRun(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handlers) ListRuns(c *gin.Context) {
	list, err := h.Svc.ListRuns(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) CreatePolicy(c *gin.Context) {
	var req dto.CreatePolicyRequest
	if !bind(c, &req) {
		return
	}
	v, err := h.Svc.CreatePolicy(ctx(c), actor(c), req.ProjectID, req.Policy)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) ListPolicies(c *gin.Context) {
	list, err := h.Svc.ListPolicies(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) EvaluateGate(c *gin.Context) {
	var req dto.GateRequest
	if !bind(c, &req) {
		return
	}
	v, err := h.Svc.EvaluateGate(ctx(c), actor(c), req.ProjectID, req.AnalysisID, req.PolicyID, req.ContractIDs, req.TestRunID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *Handlers) ListGates(c *gin.Context) {
	list, err := h.Svc.ListGates(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) Analytics(c *gin.Context) {
	v, err := h.Svc.Analytics(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handlers) InspectVersion(c *gin.Context) {
	v, err := h.Svc.InspectVersion(ctx(c), actor(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
