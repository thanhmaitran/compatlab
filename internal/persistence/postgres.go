package persistence

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// docTables maps a document kind to its table. Table names never come from
// user input.
var docTables = map[string]string{
	KindAnalysis: "analysis_runs",
	KindContract: "consumer_contracts",
	KindSuite:    "test_suites",
	KindRun:      "test_runs",
	KindPolicy:   "release_policies",
	KindReport:   "compatibility_reports",
}

// Postgres is a PostgreSQL-backed Store. All queries are parameterized.
type Postgres struct {
	pool *pgxpool.Pool
}

var _ Store = (*Postgres)(nil)

// NewPostgres connects to PostgreSQL and verifies the connection.
func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Postgres) Close()                         { p.pool.Close() }

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

// Migrate applies the *.up.sql files found in dir of fsys that have not been
// applied yet, in version order, each in its own transaction. Applied versions
// are tracked in the compatlab_migrations table.
func (p *Postgres) Migrate(ctx context.Context, fsys fs.FS, dir string) error {
	if _, err := p.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS compatlab_migrations (
        version INT PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	type migration struct {
		version int
		name    string
	}
	var list []migration
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, _ := strings.Cut(name, "_")
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("migration %q has no numeric version prefix", name)
		}
		list = append(list, migration{v, name})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	for _, m := range list {
		var one int
		err := p.pool.QueryRow(ctx, `SELECT 1 FROM compatlab_migrations WHERE version = $1`, m.version).Scan(&one)
		if err == nil {
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		sqlText, err := fs.ReadFile(fsys, dir+"/"+m.name)
		if err != nil {
			return err
		}
		tx, err := p.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO compatlab_migrations (version, name) VALUES ($1, $2)`, m.version, m.name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) CreateOrgAndUser(ctx context.Context, org *Organization, user *User) error {
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = org.CreatedAt
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name, created_at) VALUES ($1, $2, $3)`, org.ID, org.Name, org.CreatedAt); err != nil {
		return mapErr(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, org_id, email, password_hash, role, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.OrgID, user.Email, user.PasswordHash, user.Role, user.CreatedAt); err != nil {
		return mapErr(err)
	}
	return tx.Commit(ctx)
}

func (p *Postgres) CreateUser(ctx context.Context, user *User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now()
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO users (id, org_id, email, password_hash, role, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.OrgID, user.Email, user.PasswordHash, user.Role, user.CreatedAt)
	return mapErr(err)
}

func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := p.pool.QueryRow(ctx, `SELECT id, org_id, email, password_hash, role, created_at FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (p *Postgres) CreateProject(ctx context.Context, pr *Project) error {
	if pr.CreatedAt.IsZero() {
		pr.CreatedAt = now()
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO api_projects (id, org_id, name, description, created_at) VALUES ($1, $2, $3, $4, $5)`,
		pr.ID, pr.OrgID, pr.Name, pr.Description, pr.CreatedAt)
	return mapErr(err)
}

func (p *Postgres) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, org_id, name, description, created_at FROM api_projects WHERE org_id = $1 ORDER BY created_at, id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var pr Project
		if err := rows.Scan(&pr.ID, &pr.OrgID, &pr.Name, &pr.Description, &pr.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

func (p *Postgres) GetProject(ctx context.Context, orgID, id string) (*Project, error) {
	var pr Project
	err := p.pool.QueryRow(ctx, `SELECT id, org_id, name, description, created_at FROM api_projects WHERE id = $1 AND org_id = $2`, id, orgID).
		Scan(&pr.ID, &pr.OrgID, &pr.Name, &pr.Description, &pr.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pr, nil
}

func (p *Postgres) CreateSpecification(ctx context.Context, s *Specification) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now()
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO api_specifications (id, project_id, filename, format, hash, status, content, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, s.ID, s.ProjectID, s.Filename, s.Format, s.Hash, string(s.Status), s.Content, s.CreatedAt)
	return mapErr(err)
}

func (p *Postgres) GetSpecification(ctx context.Context, id string) (*Specification, error) {
	var s Specification
	var status string
	err := p.pool.QueryRow(ctx, `SELECT id, project_id, filename, format, hash, status, content, created_at FROM api_specifications WHERE id = $1`, id).
		Scan(&s.ID, &s.ProjectID, &s.Filename, &s.Format, &s.Hash, &status, &s.Content, &s.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	s.Status = domain.SpecificationStatus(status)
	return &s, nil
}

func (p *Postgres) CreateVersion(ctx context.Context, v *Version) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = now()
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO api_versions (id, project_id, label, specification_id, created_at) VALUES ($1, $2, $3, $4, $5)`,
		v.ID, v.ProjectID, v.Label, v.SpecificationID, v.CreatedAt)
	return mapErr(err)
}

func (p *Postgres) GetVersion(ctx context.Context, id string) (*Version, error) {
	var v Version
	err := p.pool.QueryRow(ctx, `SELECT id, project_id, label, specification_id, created_at FROM api_versions WHERE id = $1`, id).
		Scan(&v.ID, &v.ProjectID, &v.Label, &v.SpecificationID, &v.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &v, nil
}

func (p *Postgres) ListVersions(ctx context.Context, projectID string) ([]Version, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, project_id, label, specification_id, created_at FROM api_versions WHERE project_id = $1 ORDER BY created_at, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Label, &v.SpecificationID, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Postgres) PutDoc(ctx context.Context, d *Doc) error {
	table, ok := docTables[d.Kind]
	if !ok {
		return fmt.Errorf("unknown document kind %q", d.Kind)
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now()
	}
	_, err := p.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (id, project_id, payload, created_at) VALUES ($1, $2, $3, $4)`, table),
		d.ID, d.ProjectID, d.Payload, d.CreatedAt)
	return mapErr(err)
}

func (p *Postgres) GetDoc(ctx context.Context, kind, id string) (*Doc, error) {
	table, ok := docTables[kind]
	if !ok {
		return nil, fmt.Errorf("unknown document kind %q", kind)
	}
	d := Doc{Kind: kind}
	err := p.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, project_id, payload, created_at FROM %s WHERE id = $1`, table), id).
		Scan(&d.ID, &d.ProjectID, &d.Payload, &d.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (p *Postgres) ListDocs(ctx context.Context, kind, projectID string) ([]Doc, error) {
	table, ok := docTables[kind]
	if !ok {
		return nil, fmt.Errorf("unknown document kind %q", kind)
	}
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT id, project_id, payload, created_at FROM %s WHERE project_id = $1 ORDER BY created_at, id`, table), projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Doc{}
	for rows.Next() {
		d := Doc{Kind: kind}
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Payload, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *Postgres) SaveChanges(ctx context.Context, analysisID, projectID string, changes []domain.ClassifiedChange) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, c := range changes {
		if _, err := tx.Exec(ctx, `INSERT INTO change_records
            (analysis_id, project_id, change_id, rule_id, change_type, classification, severity, direction, method, path, location)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			analysisID, projectID, c.ID, c.RuleID, string(c.ChangeType), string(c.Classification), string(c.Severity),
			string(c.Direction), c.Method, c.Path, c.Location); err != nil {
			return mapErr(err)
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ChangeFrequency(ctx context.Context, projectID string, limit int) ([]ChangeFrequency, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := p.pool.Query(ctx, `SELECT method, path, COUNT(*) AS changes FROM change_records
        WHERE project_id = $1 AND direction = 'BACKWARD'
        GROUP BY method, path ORDER BY changes DESC, path, method LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChangeFrequency{}
	for rows.Next() {
		var c ChangeFrequency
		if err := rows.Scan(&c.Method, &c.Path, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Postgres) AddAudit(ctx context.Context, e *AuditEvent) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO audit_events (id, org_id, actor_id, action, resource, resource_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, e.OrgID, e.ActorID, e.Action, e.Resource, e.ResourceID, e.CreatedAt)
	return mapErr(err)
}
