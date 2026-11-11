// Package persistence defines the storage interface used by the backend and
// two implementations: an in-memory store (development and tests) and a
// PostgreSQL store. Only backend services import this package; the core
// engine never touches a database.
package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Sentinel errors returned by every Store implementation.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

// Document kinds stored as JSON payloads.
const (
	KindAnalysis = "analysis"
	KindContract = "contract"
	KindSuite    = "suite"
	KindRun      = "run"
	KindPolicy   = "policy"
	KindReport   = "report"
)

// Organization groups users and projects (the tenant boundary).
type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// User belongs to one organization. Role is "admin" or "member".
type User struct {
	ID           string
	OrgID        string
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

// Project is one API managed by an organization.
type Project struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"-"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Specification is an uploaded OpenAPI document.
type Specification struct {
	ID        string                     `json:"id"`
	ProjectID string                     `json:"project_id"`
	Filename  string                     `json:"filename"`
	Format    string                     `json:"format"`
	Hash      string                     `json:"hash"`
	Status    domain.SpecificationStatus `json:"status"`
	Content   []byte                     `json:"-"`
	CreatedAt time.Time                  `json:"created_at"`
}

// Version labels a specification as a release of a project.
type Version struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Label           string    `json:"label"`
	SpecificationID string    `json:"specification_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// Doc is a JSON document of one kind (analysis, contract, suite, run, policy
// or report) that belongs to a project.
type Doc struct {
	ID        string
	ProjectID string
	Kind      string
	Payload   []byte
	CreatedAt time.Time
}

// ChangeFrequency counts how often an operation changed across analyses.
type ChangeFrequency struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Count  int    `json:"count"`
}

// AuditEvent records who did what.
type AuditEvent struct {
	ID         string
	OrgID      string
	ActorID    string
	Action     string
	Resource   string
	ResourceID string
	CreatedAt  time.Time
}

// Store is the persistence boundary. Lookups that take an org ID enforce the
// tenant boundary; other entities are reached through a project that the
// caller has already authorized.
type Store interface {
	Ping(ctx context.Context) error
	Close()

	CreateOrgAndUser(ctx context.Context, org *Organization, user *User) error
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	CreateProject(ctx context.Context, p *Project) error
	ListProjects(ctx context.Context, orgID string) ([]Project, error)
	GetProject(ctx context.Context, orgID, id string) (*Project, error)

	CreateSpecification(ctx context.Context, s *Specification) error
	GetSpecification(ctx context.Context, id string) (*Specification, error)
	CreateVersion(ctx context.Context, v *Version) error
	GetVersion(ctx context.Context, id string) (*Version, error)
	ListVersions(ctx context.Context, projectID string) ([]Version, error)

	PutDoc(ctx context.Context, d *Doc) error
	GetDoc(ctx context.Context, kind, id string) (*Doc, error)
	ListDocs(ctx context.Context, kind, projectID string) ([]Doc, error)

	SaveChanges(ctx context.Context, analysisID, projectID string, changes []domain.ClassifiedChange) error
	ChangeFrequency(ctx context.Context, projectID string, limit int) ([]ChangeFrequency, error)

	AddAudit(ctx context.Context, e *AuditEvent) error
}

// validKind reports whether kind is a known document kind.
func validKind(kind string) bool {
	switch kind {
	case KindAnalysis, KindContract, KindSuite, KindRun, KindPolicy, KindReport:
		return true
	}
	return false
}
