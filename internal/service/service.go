// Package service orchestrates the core engine and the persistence layer for
// the backend. HTTP handlers call this package and contain no business logic.
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/thanhmaitran/compatlab/internal/analytics"
	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/contracts"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/generation"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/policies"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

// Errors mapped to HTTP statuses by the handlers.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Actor is the authenticated caller.
type Actor struct {
	UserID string
	OrgID  string
	Role   string
}

// Service holds the dependencies of every use case.
type Service struct {
	Store          persistence.Store
	Tokens         *auth.TokenIssuer
	Parser         *specification.Parser
	Engine         *compatibility.Engine
	Policies       policies.Evaluator
	AllowedTargets []string // hosts the test runner may contact; set by the operator, never by requests
	EngineVersion  string
}

// New creates a service with the default engine components.
func New(store persistence.Store, tokens *auth.TokenIssuer, allowedTargets []string) *Service {
	return &Service{
		Store: store, Tokens: tokens, Parser: specification.NewParser(), Engine: compatibility.NewEngine(),
		AllowedTargets: allowedTargets, EngineVersion: "0.1.0-dev",
	}
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalid, fmt.Sprintf(format, a...))
}

func (s *Service) audit(ctx context.Context, a Actor, action, resource, id string) {
	err := s.Store.AddAudit(ctx, &persistence.AuditEvent{ID: newID(), OrgID: a.OrgID, ActorID: a.UserID, Action: action, Resource: resource, ResourceID: id})
	if err != nil {
		log.Printf("audit write failed: %v", err)
	}
}

func (s *Service) project(ctx context.Context, a Actor, id string) (*persistence.Project, error) {
	return s.Store.GetProject(ctx, a.OrgID, id)
}

func (s *Service) putDoc(ctx context.Context, kind, id, projectID string, payload any) (*persistence.Doc, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if id == "" {
		id = newID()
	}
	d := &persistence.Doc{ID: id, ProjectID: projectID, Kind: kind, Payload: data}
	if err := s.Store.PutDoc(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// loadDoc fetches a document and checks that the caller's organization owns
// its project; documents of other organizations look like they do not exist.
func (s *Service) loadDoc(ctx context.Context, a Actor, kind, id string, out any) (*persistence.Doc, error) {
	d, err := s.Store.GetDoc(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, a, d.ProjectID); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(d.Payload, out); err != nil {
		return nil, err
	}
	return d, nil
}

// ---------- accounts ----------

func validEmail(e string) bool {
	at := strings.Index(e, "@")
	return len(e) <= 254 && at > 0 && at < len(e)-3 && strings.Contains(e[at:], ".") && !strings.ContainsAny(e, " \t\r\n") && strings.Count(e, "@") == 1
}

// Register creates an organization with its first user, who becomes admin.
func (s *Service) Register(ctx context.Context, orgName, email, password string) (string, error) {
	orgName, email = strings.TrimSpace(orgName), strings.ToLower(strings.TrimSpace(email))
	switch {
	case len(orgName) < 2 || len(orgName) > 100:
		return "", invalid("organization name must be 2-100 characters")
	case !validEmail(email):
		return "", invalid("a valid e-mail address is required")
	case len(password) < 8 || len(password) > auth.MaxPasswordBytes:
		return "", invalid("password must be 8-%d characters", auth.MaxPasswordBytes)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	org := &persistence.Organization{ID: newID(), Name: orgName}
	user := &persistence.User{ID: newID(), OrgID: org.ID, Email: email, PasswordHash: hash, Role: "admin"}
	if err := s.Store.CreateOrgAndUser(ctx, org, user); err != nil {
		return "", err
	}
	s.audit(ctx, Actor{UserID: user.ID, OrgID: org.ID, Role: user.Role}, "organization.register", "organization", org.ID)
	return s.Tokens.Issue(auth.Claims{Sub: user.ID, Org: org.ID, Role: user.Role})
}

// Login verifies credentials and returns a token. Failures are deliberately
// indistinguishable (unknown e-mail vs. wrong password).
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.Store.GetUserByEmail(ctx, email)
	if errors.Is(err, persistence.ErrNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return "", ErrUnauthorized
	}
	return s.Tokens.Issue(auth.Claims{Sub: u.ID, Org: u.OrgID, Role: u.Role})
}

// CreateMember lets an admin add a user to the organization.
func (s *Service) CreateMember(ctx context.Context, a Actor, email, password, role string) (*persistence.User, error) {
	if a.Role != "admin" {
		return nil, ErrForbidden
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if role == "" {
		role = "member"
	}
	switch {
	case role != "member" && role != "admin":
		return nil, invalid("role must be member or admin")
	case !validEmail(email):
		return nil, invalid("a valid e-mail address is required")
	case len(password) < 8 || len(password) > auth.MaxPasswordBytes:
		return nil, invalid("password must be 8-%d characters", auth.MaxPasswordBytes)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &persistence.User{ID: newID(), OrgID: a.OrgID, Email: email, PasswordHash: hash, Role: role}
	if err := s.Store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	s.audit(ctx, a, "user.create", "user", u.ID)
	return u, nil
}

// ---------- projects, specifications, versions ----------

// CreateProject adds a project to the caller's organization.
func (s *Service) CreateProject(ctx context.Context, a Actor, name, description string) (*persistence.Project, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 100 || len(description) > 1000 {
		return nil, invalid("project name must be 2-100 characters and the description at most 1000")
	}
	p := &persistence.Project{ID: newID(), OrgID: a.OrgID, Name: name, Description: description}
	if err := s.Store.CreateProject(ctx, p); err != nil {
		return nil, err
	}
	s.audit(ctx, a, "project.create", "project", p.ID)
	return p, nil
}

func (s *Service) ListProjects(ctx context.Context, a Actor) ([]persistence.Project, error) {
	return s.Store.ListProjects(ctx, a.OrgID)
}

func (s *Service) GetProject(ctx context.Context, a Actor, id string) (*persistence.Project, error) {
	return s.project(ctx, a, id)
}

// UploadResult describes a stored specification version.
type UploadResult struct {
	Version       persistence.Version       `json:"version"`
	Specification persistence.Specification `json:"specification"`
	Endpoints     int                       `json:"endpoints"`
	Schemas       int                       `json:"schemas"`
}

// UploadSpecification validates an OpenAPI document and stores it as a new version.
func (s *Service) UploadSpecification(ctx context.Context, a Actor, projectID, label, filename string, content []byte) (*UploadResult, error) {
	proj, err := s.project(ctx, a, projectID)
	if err != nil {
		return nil, err
	}
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 64 {
		return nil, invalid("version label must be 1-64 characters")
	}
	if len(filename) > 255 {
		return nil, invalid("filename is too long")
	}
	api, err := s.Parser.Parse(content)
	if err != nil {
		return nil, err
	}
	format := "yaml"
	if t := bytes.TrimSpace(content); len(t) > 0 && t[0] == '{' {
		format = "json"
	}
	sum := sha256.Sum256(content)
	spec := &persistence.Specification{
		ID: newID(), ProjectID: proj.ID, Filename: filename, Format: format,
		Hash: hex.EncodeToString(sum[:]), Status: domain.SpecValid, Content: content,
	}
	if err := s.Store.CreateSpecification(ctx, spec); err != nil {
		return nil, err
	}
	v := &persistence.Version{ID: newID(), ProjectID: proj.ID, Label: label, SpecificationID: spec.ID}
	if err := s.Store.CreateVersion(ctx, v); err != nil {
		return nil, err
	}
	s.audit(ctx, a, "specification.upload", "version", v.ID)
	return &UploadResult{Version: *v, Specification: *spec, Endpoints: len(api.Endpoints), Schemas: len(api.Schemas)}, nil
}

func (s *Service) ListVersions(ctx context.Context, a Actor, projectID string) ([]persistence.Version, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	return s.Store.ListVersions(ctx, projectID)
}

// versionAPI loads a version of a project the caller owns and parses its spec.
func (s *Service) versionAPI(ctx context.Context, a Actor, projectID, versionID string) (*persistence.Version, *domain.NormalizedAPI, error) {
	v, err := s.Store.GetVersion(ctx, versionID)
	if err != nil {
		return nil, nil, err
	}
	if projectID != "" && v.ProjectID != projectID {
		return nil, nil, persistence.ErrNotFound
	}
	if _, err := s.project(ctx, a, v.ProjectID); err != nil {
		return nil, nil, err
	}
	spec, err := s.Store.GetSpecification(ctx, v.SpecificationID)
	if err != nil {
		return nil, nil, err
	}
	api, err := s.Parser.Parse(spec.Content)
	if err != nil {
		return nil, nil, err
	}
	return v, api, nil
}

// ---------- analysis ----------

type analysisDoc struct {
	BaseVersionID   string                        `json:"base_version_id"`
	TargetVersionID string                        `json:"target_version_id"`
	Analysis        *domain.CompatibilityAnalysis `json:"analysis"`
}

// AnalysisView is the API representation of a stored analysis.
type AnalysisView struct {
	ID              string                        `json:"id"`
	ProjectID       string                        `json:"project_id"`
	BaseVersionID   string                        `json:"base_version_id"`
	TargetVersionID string                        `json:"target_version_id"`
	CreatedAt       time.Time                     `json:"created_at"`
	Scores          *domain.CompatibilityScores   `json:"scores,omitempty"`
	Backward        domain.AnalysisSummary        `json:"backward"`
	Forward         domain.AnalysisSummary        `json:"forward"`
	Analysis        *domain.CompatibilityAnalysis `json:"analysis,omitempty"`
}

func analysisView(d *persistence.Doc, doc analysisDoc, withBody bool) AnalysisView {
	v := AnalysisView{ID: d.ID, ProjectID: d.ProjectID, BaseVersionID: doc.BaseVersionID, TargetVersionID: doc.TargetVersionID, CreatedAt: d.CreatedAt}
	if doc.Analysis != nil {
		v.Scores = doc.Analysis.Scores
		v.Backward = doc.Analysis.SummaryFor(domain.DirectionBackward)
		v.Forward = doc.Analysis.SummaryFor(domain.DirectionForward)
		if withBody {
			v.Analysis = doc.Analysis
		}
	}
	return v
}

// Analyze compares two versions of a project, stores and returns the result.
func (s *Service) Analyze(ctx context.Context, a Actor, projectID, baseID, targetID string) (*AnalysisView, error) {
	proj, err := s.project(ctx, a, projectID)
	if err != nil {
		return nil, err
	}
	if baseID == targetID {
		return nil, invalid("base and target versions must differ")
	}
	bv, baseAPI, err := s.versionAPI(ctx, a, proj.ID, baseID)
	if err != nil {
		return nil, err
	}
	tv, targetAPI, err := s.versionAPI(ctx, a, proj.ID, targetID)
	if err != nil {
		return nil, err
	}
	analysis, err := s.Engine.Compare(baseAPI, targetAPI)
	if err != nil {
		return nil, err
	}
	analysis.BaseVersion, analysis.TargetVersion = bv.Label, tv.Label
	payload := analysisDoc{BaseVersionID: bv.ID, TargetVersionID: tv.ID, Analysis: analysis}
	d, err := s.putDoc(ctx, persistence.KindAnalysis, "", proj.ID, payload)
	if err != nil {
		return nil, err
	}
	if err := s.Store.SaveChanges(ctx, d.ID, proj.ID, analysis.Changes); err != nil {
		return nil, err
	}
	s.audit(ctx, a, "analysis.create", "analysis", d.ID)
	v := analysisView(d, payload, true)
	return &v, nil
}

func (s *Service) loadAnalysis(ctx context.Context, a Actor, id string) (*persistence.Doc, analysisDoc, error) {
	var doc analysisDoc
	d, err := s.loadDoc(ctx, a, persistence.KindAnalysis, id, &doc)
	return d, doc, err
}

func (s *Service) GetAnalysis(ctx context.Context, a Actor, id string) (*AnalysisView, error) {
	d, doc, err := s.loadAnalysis(ctx, a, id)
	if err != nil {
		return nil, err
	}
	v := analysisView(d, doc, true)
	return &v, nil
}

// AnalysisChanges returns the changes of an analysis, optionally filtered.
func (s *Service) AnalysisChanges(ctx context.Context, a Actor, id, classification, direction string) ([]domain.ClassifiedChange, error) {
	_, doc, err := s.loadAnalysis(ctx, a, id)
	if err != nil {
		return nil, err
	}
	if classification != "" && !domain.ChangeClassification(classification).Valid() {
		return nil, invalid("unknown classification %q", classification)
	}
	if direction != "" && !domain.CompatibilityDirection(direction).Valid() {
		return nil, invalid("unknown direction %q", direction)
	}
	out := []domain.ClassifiedChange{}
	if doc.Analysis == nil {
		return out, nil
	}
	for _, c := range doc.Analysis.Changes {
		if classification != "" && string(c.Classification) != classification {
			continue
		}
		if direction != "" && string(c.Direction) != direction {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) ListAnalyses(ctx context.Context, a Actor, projectID string) ([]AnalysisView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	docs, err := s.Store.ListDocs(ctx, persistence.KindAnalysis, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]AnalysisView, 0, len(docs))
	for i := range docs {
		var doc analysisDoc
		if err := json.Unmarshal(docs[i].Payload, &doc); err != nil {
			return nil, err
		}
		out = append(out, analysisView(&docs[i], doc, false))
	}
	return out, nil
}

// ---------- contracts ----------

// ContractView is the API representation of a stored consumer contract.
type ContractView struct {
	ID           string                   `json:"id"`
	ProjectID    string                   `json:"project_id"`
	Consumer     string                   `json:"consumer"`
	Provider     string                   `json:"provider"`
	Interactions int                      `json:"interactions"`
	CreatedAt    time.Time                `json:"created_at"`
	Contract     *domain.ConsumerContract `json:"contract,omitempty"`
}

func contractView(d *persistence.Doc, c *domain.ConsumerContract, withBody bool) ContractView {
	v := ContractView{ID: d.ID, ProjectID: d.ProjectID, Consumer: c.Consumer, Provider: c.Provider, Interactions: len(c.Interactions), CreatedAt: d.CreatedAt}
	if withBody {
		v.Contract = c
	}
	return v
}

func (s *Service) RegisterContract(ctx context.Context, a Actor, projectID string, c *domain.ConsumerContract) (*ContractView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	if c == nil {
		return nil, invalid("contract is required")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	d, err := s.putDoc(ctx, persistence.KindContract, "", projectID, c)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, a, "contract.register", "contract", d.ID)
	v := contractView(d, c, true)
	return &v, nil
}

func (s *Service) ListContracts(ctx context.Context, a Actor, projectID string) ([]ContractView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	docs, err := s.Store.ListDocs(ctx, persistence.KindContract, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]ContractView, 0, len(docs))
	for i := range docs {
		var c domain.ConsumerContract
		if err := json.Unmarshal(docs[i].Payload, &c); err != nil {
			return nil, err
		}
		out = append(out, contractView(&docs[i], &c, false))
	}
	return out, nil
}

// VerifyResult is the outcome of verifying a contract against a version.
type VerifyResult struct {
	Passed       bool                         `json:"passed"`
	Verification *domain.ContractVerification `json:"verification"`
}

func (s *Service) VerifyContract(ctx context.Context, a Actor, contractID, versionID string) (*VerifyResult, error) {
	var c domain.ConsumerContract
	d, err := s.loadDoc(ctx, a, persistence.KindContract, contractID, &c)
	if err != nil {
		return nil, err
	}
	_, api, err := s.versionAPI(ctx, a, d.ProjectID, versionID)
	if err != nil {
		return nil, err
	}
	res, err := contracts.NewVerifier().Verify(&c, api)
	if err != nil {
		return nil, err
	}
	return &VerifyResult{Passed: res.Passed(), Verification: res}, nil
}

// ---------- test suites and runs ----------

type suiteDoc struct {
	VersionID string            `json:"version_id"`
	Suite     *domain.TestSuite `json:"suite"`
}

// SuiteView is the API representation of a generated suite.
type SuiteView struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"project_id"`
	VersionID string            `json:"version_id"`
	Name      string            `json:"name"`
	Cases     int               `json:"cases"`
	CreatedAt time.Time         `json:"created_at"`
	Suite     *domain.TestSuite `json:"suite,omitempty"`
}

func suiteView(d *persistence.Doc, doc suiteDoc, withBody bool) SuiteView {
	v := SuiteView{ID: d.ID, ProjectID: d.ProjectID, VersionID: doc.VersionID, CreatedAt: d.CreatedAt}
	if doc.Suite != nil {
		v.Name, v.Cases = doc.Suite.Name, len(doc.Suite.Cases)
		if withBody {
			v.Suite = doc.Suite
		}
	}
	return v
}

// priorityEndpoints lists operations touched by backward breaking or
// conditional changes so their tests run first.
func priorityEndpoints(an *domain.CompatibilityAnalysis) []string {
	set := map[string]bool{}
	if an != nil {
		for _, c := range an.Changes {
			if c.Direction == domain.DirectionBackward && c.Classification != domain.NonBreaking && c.Method != "" {
				set[strings.ToUpper(c.Method)+" "+domain.CanonicalPath(c.Path)] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Service) GenerateSuite(ctx context.Context, a Actor, versionID, analysisID string, cfg domain.GenerationConfig) (*SuiteView, error) {
	v, api, err := s.versionAPI(ctx, a, "", versionID)
	if err != nil {
		return nil, err
	}
	if analysisID != "" {
		d, doc, err := s.loadAnalysis(ctx, a, analysisID)
		if err != nil {
			return nil, err
		}
		if d.ProjectID != v.ProjectID {
			return nil, persistence.ErrNotFound
		}
		cfg.PriorityEndpoints = priorityEndpoints(doc.Analysis)
	}
	suite, err := generation.Generator{}.Generate(api, cfg)
	if err != nil {
		return nil, err
	}
	payload := suiteDoc{VersionID: v.ID, Suite: suite}
	d, err := s.putDoc(ctx, persistence.KindSuite, "", v.ProjectID, payload)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, a, "suite.generate", "suite", d.ID)
	view := suiteView(d, payload, true)
	return &view, nil
}

func (s *Service) GetSuite(ctx context.Context, a Actor, id string) (*SuiteView, error) {
	var doc suiteDoc
	d, err := s.loadDoc(ctx, a, persistence.KindSuite, id, &doc)
	if err != nil {
		return nil, err
	}
	v := suiteView(d, doc, true)
	return &v, nil
}

type runDoc struct {
	SuiteID   string          `json:"suite_id"`
	VersionID string          `json:"version_id"`
	Target    string          `json:"target"`
	Run       *domain.TestRun `json:"run"`
}

// RunView is the API representation of a test run.
type RunView struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	SuiteID   string          `json:"suite_id"`
	VersionID string          `json:"version_id"`
	Target    string          `json:"target"`
	PassRate  float64         `json:"pass_rate"`
	Passed    int             `json:"passed"`
	Failed    int             `json:"failed"`
	Errored   int             `json:"errored"`
	CreatedAt time.Time       `json:"created_at"`
	Run       *domain.TestRun `json:"run,omitempty"`
}

func runView(d *persistence.Doc, doc runDoc, withBody bool) RunView {
	v := RunView{ID: d.ID, ProjectID: d.ProjectID, SuiteID: doc.SuiteID, VersionID: doc.VersionID, Target: doc.Target, CreatedAt: d.CreatedAt}
	if doc.Run != nil {
		v.PassRate = doc.Run.PassRate()
		for _, r := range doc.Run.Results {
			switch r.Status {
			case domain.TestStatusPassed:
				v.Passed++
			case domain.TestStatusFailed:
				v.Failed++
			case domain.TestStatusErrored:
				v.Errored++
			}
		}
		if withBody {
			v.Run = doc.Run
		}
	}
	return v
}

// RunTests executes a suite against an in-process mock of the version, or, if
// target is set, against that target. Targets must be on the operator's allowlist.
func (s *Service) RunTests(ctx context.Context, a Actor, suiteID, versionID, target string) (*RunView, error) {
	var sd suiteDoc
	suiteD, err := s.loadDoc(ctx, a, persistence.KindSuite, suiteID, &sd)
	if err != nil {
		return nil, err
	}
	v, api, err := s.versionAPI(ctx, a, suiteD.ProjectID, versionID)
	if err != nil {
		return nil, err
	}
	if sd.Suite == nil {
		return nil, invalid("the suite is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var run *domain.TestRun
	if target == "" {
		run, err = execution.RunOnMock(ctx, api, sd.Suite)
	} else {
		runner := execution.Runner{Config: execution.Config{Policy: security.TargetPolicy{AllowedHosts: s.AllowedTargets}}}
		run, err = runner.Run(ctx, api, sd.Suite, target)
	}
	if err != nil {
		return nil, err
	}
	payload := runDoc{SuiteID: suiteD.ID, VersionID: v.ID, Target: target, Run: run}
	d, err := s.putDoc(ctx, persistence.KindRun, "", suiteD.ProjectID, payload)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, a, "testrun.create", "testrun", d.ID)
	view := runView(d, payload, true)
	return &view, nil
}

func (s *Service) GetRun(ctx context.Context, a Actor, id string) (*RunView, error) {
	var doc runDoc
	d, err := s.loadDoc(ctx, a, persistence.KindRun, id, &doc)
	if err != nil {
		return nil, err
	}
	v := runView(d, doc, true)
	return &v, nil
}

func (s *Service) ListRuns(ctx context.Context, a Actor, projectID string) ([]RunView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	docs, err := s.Store.ListDocs(ctx, persistence.KindRun, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]RunView, 0, len(docs))
	for i := range docs {
		var doc runDoc
		if err := json.Unmarshal(docs[i].Payload, &doc); err != nil {
			return nil, err
		}
		out = append(out, runView(&docs[i], doc, false))
	}
	return out, nil
}

// ---------- policies and the release gate ----------

// PolicyView is the API representation of a stored release policy.
type PolicyView struct {
	ID        string                `json:"id"`
	ProjectID string                `json:"project_id"`
	CreatedAt time.Time             `json:"created_at"`
	Policy    *domain.ReleasePolicy `json:"policy"`
}

// CreatePolicy stores a release policy. Only admins may create policies.
func (s *Service) CreatePolicy(ctx context.Context, a Actor, projectID string, p *domain.ReleasePolicy) (*PolicyView, error) {
	if a.Role != "admin" {
		return nil, ErrForbidden
	}
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, invalid("policy is required")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	id := newID()
	p.ID = id
	d, err := s.putDoc(ctx, persistence.KindPolicy, id, projectID, p)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, a, "policy.create", "policy", id)
	return &PolicyView{ID: id, ProjectID: projectID, CreatedAt: d.CreatedAt, Policy: p}, nil
}

func (s *Service) ListPolicies(ctx context.Context, a Actor, projectID string) ([]PolicyView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	docs, err := s.Store.ListDocs(ctx, persistence.KindPolicy, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]PolicyView, 0, len(docs))
	for i := range docs {
		var p domain.ReleasePolicy
		if err := json.Unmarshal(docs[i].Payload, &p); err != nil {
			return nil, err
		}
		out = append(out, PolicyView{ID: docs[i].ID, ProjectID: docs[i].ProjectID, CreatedAt: docs[i].CreatedAt, Policy: &p})
	}
	return out, nil
}

type reportDoc struct {
	AnalysisID string                      `json:"analysis_id"`
	PolicyID   string                      `json:"policy_id"`
	Report     *domain.CompatibilityReport `json:"report"`
}

// GateView is the API representation of a release-gate evaluation.
type GateView struct {
	ID         string                      `json:"id"`
	ProjectID  string                      `json:"project_id"`
	AnalysisID string                      `json:"analysis_id"`
	PolicyID   string                      `json:"policy_id"`
	CreatedAt  time.Time                   `json:"created_at"`
	Report     *domain.CompatibilityReport `json:"report"`
}

// EvaluateGate evaluates a release policy against a stored analysis, the
// listed contracts (verified against the analysis' target version) and an
// optional test run, and stores the resulting report.
func (s *Service) EvaluateGate(ctx context.Context, a Actor, projectID, analysisID, policyID string, contractIDs []string, runID string) (*GateView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	ad, adoc, err := s.loadAnalysis(ctx, a, analysisID)
	if err != nil {
		return nil, err
	}
	if ad.ProjectID != projectID || adoc.Analysis == nil {
		return nil, persistence.ErrNotFound
	}
	var policy domain.ReleasePolicy
	pd, err := s.loadDoc(ctx, a, persistence.KindPolicy, policyID, &policy)
	if err != nil {
		return nil, err
	}
	if pd.ProjectID != projectID {
		return nil, persistence.ErrNotFound
	}
	report := &domain.CompatibilityReport{Analysis: adoc.Analysis, EngineVersion: s.EngineVersion}
	if len(contractIDs) > 0 {
		_, targetAPI, err := s.versionAPI(ctx, a, projectID, adoc.TargetVersionID)
		if err != nil {
			return nil, err
		}
		var loaded []*domain.ConsumerContract
		for _, id := range contractIDs {
			c := &domain.ConsumerContract{}
			cd, err := s.loadDoc(ctx, a, persistence.KindContract, id, c)
			if err != nil {
				return nil, err
			}
			if cd.ProjectID != projectID {
				return nil, persistence.ErrNotFound
			}
			res, err := contracts.NewVerifier().Verify(c, targetAPI)
			if err != nil {
				return nil, err
			}
			loaded = append(loaded, c)
			report.Contracts = append(report.Contracts, *res)
		}
		report.Impacts = contracts.Impact(adoc.Analysis, loaded)
	}
	if runID != "" {
		var rd runDoc
		d, err := s.loadDoc(ctx, a, persistence.KindRun, runID, &rd)
		if err != nil {
			return nil, err
		}
		if d.ProjectID != projectID {
			return nil, persistence.ErrNotFound
		}
		report.TestRun = rd.Run
	}
	eval, err := s.Policies.Evaluate(&policy, report)
	if err != nil {
		return nil, err
	}
	report.Evaluation = eval
	rdoc, err := s.putDoc(ctx, persistence.KindReport, "", projectID, reportDoc{AnalysisID: analysisID, PolicyID: policyID, Report: report})
	if err != nil {
		return nil, err
	}
	s.audit(ctx, a, "gate.evaluate", "report", rdoc.ID)
	return &GateView{ID: rdoc.ID, ProjectID: projectID, AnalysisID: analysisID, PolicyID: policyID, CreatedAt: rdoc.CreatedAt, Report: report}, nil
}

func (s *Service) ListGates(ctx context.Context, a Actor, projectID string) ([]GateView, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	docs, err := s.Store.ListDocs(ctx, persistence.KindReport, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]GateView, 0, len(docs))
	for i := range docs {
		var rd reportDoc
		if err := json.Unmarshal(docs[i].Payload, &rd); err != nil {
			return nil, err
		}
		out = append(out, GateView{ID: docs[i].ID, ProjectID: docs[i].ProjectID, AnalysisID: rd.AnalysisID, PolicyID: rd.PolicyID, CreatedAt: docs[i].CreatedAt, Report: rd.Report})
	}
	return out, nil
}

// ---------- analytics ----------

// Analytics computes the historical metrics of a project.
func (s *Service) Analytics(ctx context.Context, a Actor, projectID string) (*analytics.Summary, error) {
	if _, err := s.project(ctx, a, projectID); err != nil {
		return nil, err
	}
	versions, err := s.Store.ListVersions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{}
	for _, v := range versions {
		labels[v.ID] = v.Label
	}
	label := func(id string) string {
		if l, ok := labels[id]; ok {
			return l
		}
		return id
	}
	var in analytics.Input
	docs, err := s.Store.ListDocs(ctx, persistence.KindAnalysis, projectID)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		var doc analysisDoc
		if err := json.Unmarshal(d.Payload, &doc); err != nil {
			return nil, err
		}
		in.Analyses = append(in.Analyses, analytics.AnalysisPoint{
			ID: d.ID, BaseLabel: label(doc.BaseVersionID), TargetLabel: label(doc.TargetVersionID), CreatedAt: d.CreatedAt, Analysis: doc.Analysis})
	}
	if docs, err = s.Store.ListDocs(ctx, persistence.KindRun, projectID); err != nil {
		return nil, err
	}
	for _, d := range docs {
		var doc runDoc
		if err := json.Unmarshal(d.Payload, &doc); err != nil {
			return nil, err
		}
		in.Runs = append(in.Runs, analytics.RunPoint{ID: d.ID, CreatedAt: d.CreatedAt, Run: doc.Run})
	}
	if docs, err = s.Store.ListDocs(ctx, persistence.KindReport, projectID); err != nil {
		return nil, err
	}
	for _, d := range docs {
		var doc reportDoc
		if err := json.Unmarshal(d.Payload, &doc); err != nil {
			return nil, err
		}
		in.Reports = append(in.Reports, analytics.ReportPoint{ID: d.ID, CreatedAt: d.CreatedAt, Report: doc.Report})
	}
	freq, err := s.Store.ChangeFrequency(ctx, projectID, 10)
	if err != nil {
		return nil, err
	}
	for _, f := range freq {
		in.TopEndpoints = append(in.TopEndpoints, analytics.EndpointChange{Method: f.Method, Path: f.Path, Count: f.Count})
	}
	summary := analytics.Compute(in)
	return &summary, nil
}

// ---------- specification explorer ----------

// SpecParameter summarizes a parameter for the explorer.
type SpecParameter struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
}

// SpecEndpoint summarizes an operation for the explorer.
type SpecEndpoint struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	OperationID string          `json:"operation_id,omitempty"`
	Deprecated  bool            `json:"deprecated,omitempty"`
	Parameters  []SpecParameter `json:"parameters"`
	Responses   []string        `json:"responses"`
	Security    []string        `json:"security"`
	RequestBody string          `json:"request_body,omitempty"`
}

// SpecSchema summarizes a component schema (without following references,
// which may be circular).
type SpecSchema struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Properties []string `json:"properties"`
	Required   []string `json:"required"`
}

// SpecView is a flat, JSON-safe description of a stored specification.
type SpecView struct {
	Version         persistence.Version `json:"version"`
	Title           string              `json:"title"`
	APIVersion      string              `json:"api_version"`
	OpenAPIVersion  string              `json:"openapi_version"`
	Endpoints       []SpecEndpoint      `json:"endpoints"`
	Schemas         []SpecSchema        `json:"schemas"`
	SecuritySchemes []string            `json:"security_schemes"`
}

// InspectVersion returns a summary of the paths, operations and schemas of a version.
func (s *Service) InspectVersion(ctx context.Context, a Actor, versionID string) (*SpecView, error) {
	v, api, err := s.versionAPI(ctx, a, "", versionID)
	if err != nil {
		return nil, err
	}
	view := &SpecView{
		Version: *v, Title: api.Title, APIVersion: api.Version, OpenAPIVersion: api.OpenAPIVersion,
		Endpoints: []SpecEndpoint{}, Schemas: []SpecSchema{}, SecuritySchemes: append([]string{}, api.SecuritySchemes...),
	}
	for _, e := range api.Endpoints {
		se := SpecEndpoint{Method: e.Method, Path: e.Path, OperationID: e.OperationID, Deprecated: e.Deprecated,
			Parameters: []SpecParameter{}, Responses: []string{}, Security: []string{}}
		for _, p := range e.Parameters {
			typ := ""
			if p.Schema != nil {
				typ = string(p.Schema.Type)
			}
			se.Parameters = append(se.Parameters, SpecParameter{Name: p.Name, Location: string(p.Location), Required: p.Required, Type: typ})
		}
		for _, r := range e.Responses {
			se.Responses = append(se.Responses, r.StatusCode)
		}
		for _, sec := range e.Security {
			se.Security = append(se.Security, sec.Name)
		}
		if e.RequestBody != nil {
			se.RequestBody = e.RequestBody.ContentType
		}
		view.Endpoints = append(view.Endpoints, se)
	}
	names := make([]string, 0, len(api.Schemas))
	for n := range api.Schemas {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sc := api.Schemas[n]
		view.Schemas = append(view.Schemas, SpecSchema{Name: n, Type: string(sc.Type), Properties: append([]string{}, sc.PropertyNames()...), Required: append([]string{}, sc.Required...)})
	}
	return view, nil
}
