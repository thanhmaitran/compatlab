package persistence

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

type changeRow struct {
	analysisID, projectID string
	change                domain.ClassifiedChange
}

// Memory is a thread-safe in-memory Store for development and tests. Data is
// lost when the process exits.
type Memory struct {
	mu       sync.RWMutex
	orgs     map[string]Organization
	users    map[string]User // by email
	projects []Project
	specs    map[string]Specification
	versions []Version
	docs     []Doc
	changes  []changeRow
	audits   []AuditEvent
}

var _ Store = (*Memory)(nil)

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{orgs: map[string]Organization{}, users: map[string]User{}, specs: map[string]Specification{}}
}

func now() time.Time { return time.Now().UTC() }

// Ping always succeeds.
func (m *Memory) Ping(context.Context) error { return nil }

// Close is a no-op.
func (m *Memory) Close() {}

func (m *Memory) CreateOrgAndUser(_ context.Context, org *Organization, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[user.Email]; exists {
		return ErrConflict
	}
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now()
	}
	m.orgs[org.ID] = *org
	m.users[user.Email] = *user
	return nil
}

func (m *Memory) CreateUser(_ context.Context, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[user.Email]; exists {
		return ErrConflict
	}
	if _, ok := m.orgs[user.OrgID]; !ok {
		return ErrNotFound
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now()
	}
	m.users[user.Email] = *user
	return nil
}

func (m *Memory) GetUserByEmail(_ context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[email]
	if !ok {
		return nil, ErrNotFound
	}
	return &u, nil
}

func (m *Memory) CreateProject(_ context.Context, p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.projects {
		if e.OrgID == p.OrgID && e.Name == p.Name {
			return ErrConflict
		}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now()
	}
	m.projects = append(m.projects, *p)
	return nil
}

func (m *Memory) ListProjects(_ context.Context, orgID string) ([]Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Project{}
	for _, p := range m.projects {
		if p.OrgID == orgID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *Memory) GetProject(_ context.Context, orgID, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.projects {
		if p.ID == id && p.OrgID == orgID {
			c := p
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) CreateSpecification(_ context.Context, s *Specification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.specs[s.ID]; exists {
		return ErrConflict
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now()
	}
	m.specs[s.ID] = *s
	return nil
}

func (m *Memory) GetSpecification(_ context.Context, id string) (*Specification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.specs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &s, nil
}

func (m *Memory) CreateVersion(_ context.Context, v *Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.versions {
		if e.ProjectID == v.ProjectID && e.Label == v.Label {
			return ErrConflict
		}
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = now()
	}
	m.versions = append(m.versions, *v)
	return nil
}

func (m *Memory) GetVersion(_ context.Context, id string) (*Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.versions {
		if v.ID == id {
			c := v
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) ListVersions(_ context.Context, projectID string) ([]Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Version{}
	for _, v := range m.versions {
		if v.ProjectID == projectID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *Memory) PutDoc(_ context.Context, d *Doc) error {
	if !validKind(d.Kind) {
		return fmt.Errorf("unknown document kind %q", d.Kind)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.docs {
		if e.Kind == d.Kind && e.ID == d.ID {
			return ErrConflict
		}
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now()
	}
	c := *d
	c.Payload = append([]byte(nil), d.Payload...)
	m.docs = append(m.docs, c)
	return nil
}

func (m *Memory) GetDoc(_ context.Context, kind, id string) (*Doc, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.docs {
		if d.Kind == kind && d.ID == id {
			c := d
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) ListDocs(_ context.Context, kind, projectID string) ([]Doc, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Doc{}
	for _, d := range m.docs {
		if d.Kind == kind && d.ProjectID == projectID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *Memory) SaveChanges(_ context.Context, analysisID, projectID string, changes []domain.ClassifiedChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range changes {
		m.changes = append(m.changes, changeRow{analysisID, projectID, c})
	}
	return nil
}

func (m *Memory) ChangeFrequency(_ context.Context, projectID string, limit int) ([]ChangeFrequency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := map[string]*ChangeFrequency{}
	for _, r := range m.changes {
		if r.projectID != projectID || r.change.Direction != domain.DirectionBackward {
			continue
		}
		key := r.change.Method + " " + r.change.Path
		if counts[key] == nil {
			counts[key] = &ChangeFrequency{Method: r.change.Method, Path: r.change.Path}
		}
		counts[key].Count++
	}
	out := make([]ChangeFrequency, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Path != out[j].Path {
			return strings.Compare(out[i].Path, out[j].Path) < 0
		}
		return out[i].Method < out[j].Method
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) AddAudit(_ context.Context, e *AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now()
	}
	m.audits = append(m.audits, *e)
	return nil
}

// AuditEvents returns the recorded audit trail (used by tests).
func (m *Memory) AuditEvents() []AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]AuditEvent(nil), m.audits...)
}
