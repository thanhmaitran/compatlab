package persistence

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// storeContract runs the behaviour every Store implementation must provide.
// suffix keeps IDs and e-mail addresses unique across runs on a shared database.
func storeContract(t *testing.T, s Store, suffix string) {
	t.Helper()
	ctx := context.Background()
	id := func(name string) string { return name + "-" + suffix }

	orgA := &Organization{ID: id("orgA"), Name: "Org A"}
	userA := &User{ID: id("userA"), OrgID: orgA.ID, Email: id("a") + "@example.com", PasswordHash: "hash", Role: "admin"}
	if err := s.CreateOrgAndUser(ctx, orgA, userA); err != nil {
		t.Fatalf("CreateOrgAndUser: %v", err)
	}
	dupOrg := &Organization{ID: id("orgDup"), Name: "Dup"}
	dupUser := &User{ID: id("userDup"), OrgID: dupOrg.ID, Email: userA.Email, PasswordHash: "x", Role: "admin"}
	if err := s.CreateOrgAndUser(ctx, dupOrg, dupUser); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate e-mail must conflict, got %v", err)
	}
	orgB := &Organization{ID: id("orgB"), Name: "Org B"}
	userB := &User{ID: id("userB"), OrgID: orgB.ID, Email: id("b") + "@example.com", PasswordHash: "hash", Role: "admin"}
	if err := s.CreateOrgAndUser(ctx, orgB, userB); err != nil {
		t.Fatal(err)
	}
	member := &User{ID: id("member"), OrgID: orgA.ID, Email: id("m") + "@example.com", PasswordHash: "hash", Role: "member"}
	if err := s.CreateUser(ctx, member); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := s.GetUserByEmail(ctx, userA.Email)
	if err != nil || got.ID != userA.ID || got.Role != "admin" {
		t.Fatalf("GetUserByEmail: %+v %v", got, err)
	}
	if _, err := s.GetUserByEmail(ctx, "nobody-"+suffix+"@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}

	proj := &Project{ID: id("proj"), OrgID: orgA.ID, Name: "Payments", Description: "d"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(ctx, &Project{ID: id("proj2"), OrgID: orgA.ID, Name: "Payments"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate project name in an org must conflict, got %v", err)
	}
	if _, err := s.GetProject(ctx, orgB.ID, proj.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another organization must not see the project, got %v", err)
	}
	if p, err := s.GetProject(ctx, orgA.ID, proj.ID); err != nil || p.Name != "Payments" {
		t.Errorf("GetProject: %+v %v", p, err)
	}
	if list, err := s.ListProjects(ctx, orgB.ID); err != nil || len(list) != 0 {
		t.Errorf("org B must have no projects: %v %v", list, err)
	}

	spec := &Specification{ID: id("spec"), ProjectID: proj.ID, Filename: "a.yaml", Format: "yaml", Hash: "h", Status: domain.SpecValid, Content: []byte("openapi: 3.0.3")}
	if err := s.CreateSpecification(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if gs, err := s.GetSpecification(ctx, spec.ID); err != nil || string(gs.Content) != "openapi: 3.0.3" || gs.Status != domain.SpecValid {
		t.Errorf("GetSpecification: %+v %v", gs, err)
	}
	v1 := &Version{ID: id("v1"), ProjectID: proj.ID, Label: "v1", SpecificationID: spec.ID}
	if err := s.CreateVersion(ctx, v1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVersion(ctx, &Version{ID: id("v1b"), ProjectID: proj.ID, Label: "v1", SpecificationID: spec.ID}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate version label must conflict, got %v", err)
	}
	if vs, err := s.ListVersions(ctx, proj.ID); err != nil || len(vs) != 1 || vs[0].Label != "v1" {
		t.Errorf("ListVersions: %v %v", vs, err)
	}
	if _, err := s.GetVersion(ctx, "missing-"+suffix); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing version: %v", err)
	}

	for i := 1; i <= 3; i++ {
		d := &Doc{ID: fmt.Sprintf("d%d-%s", i, suffix), ProjectID: proj.ID, Kind: KindAnalysis, Payload: []byte(fmt.Sprintf(`{"n":%d}`, i))}
		if err := s.PutDoc(ctx, d); err != nil {
			t.Fatalf("PutDoc: %v", err)
		}
	}
	if err := s.PutDoc(ctx, &Doc{ID: id("run1"), ProjectID: proj.ID, Kind: KindRun, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	docs, err := s.ListDocs(ctx, KindAnalysis, proj.ID)
	if err != nil || len(docs) != 3 || docs[0].ID != "d1-"+suffix || docs[2].ID != "d3-"+suffix {
		t.Errorf("ListDocs must be oldest first and filtered by kind: %v %v", docs, err)
	}
	if d, err := s.GetDoc(ctx, KindAnalysis, "d2-"+suffix); err != nil || string(d.Payload) != `{"n":2}` && string(d.Payload) != `{"n": 2}` {
		t.Errorf("GetDoc: %+v %v", d, err)
	}
	if _, err := s.GetDoc(ctx, KindRun, "d2-"+suffix); !errors.Is(err, ErrNotFound) {
		t.Errorf("kinds must be separate: %v", err)
	}
	if err := s.PutDoc(ctx, &Doc{ID: "x", ProjectID: proj.ID, Kind: "bogus", Payload: []byte(`{}`)}); err == nil {
		t.Error("unknown kind must be rejected")
	}

	changes := []domain.ClassifiedChange{
		{ChangeRecord: domain.ChangeRecord{ID: "c1", Method: "GET", Path: "/users", ChangeType: domain.ChangeFieldRemoved, Location: "response.200.body.email"},
			RuleID: "RES-001", Classification: domain.Breaking, Severity: domain.SeverityHigh, Direction: domain.DirectionBackward},
		{ChangeRecord: domain.ChangeRecord{ID: "c2", Method: "GET", Path: "/users", ChangeType: domain.ChangeFieldAdded, Location: "response.200.body.x"},
			RuleID: "RES-002", Classification: domain.Conditional, Severity: domain.SeverityLow, Direction: domain.DirectionBackward},
		{ChangeRecord: domain.ChangeRecord{ID: "c3", Method: "POST", Path: "/orders", ChangeType: domain.ChangeFieldAdded, Location: "request.body.y"},
			RuleID: "REQ-001", Classification: domain.Breaking, Severity: domain.SeverityHigh, Direction: domain.DirectionBackward},
		{ChangeRecord: domain.ChangeRecord{ID: "c4", Method: "POST", Path: "/orders", ChangeType: domain.ChangeFieldAdded, Location: "request.body.y"},
			RuleID: "REQ-001", Classification: domain.Conditional, Severity: domain.SeverityLow, Direction: domain.DirectionForward},
	}
	if err := s.SaveChanges(ctx, "d1-"+suffix, proj.ID, changes); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	freq, err := s.ChangeFrequency(ctx, proj.ID, 10)
	if err != nil || len(freq) != 2 || freq[0].Path != "/users" || freq[0].Count != 2 || freq[1].Path != "/orders" || freq[1].Count != 1 {
		t.Errorf("ChangeFrequency must count backward changes per operation: %+v %v", freq, err)
	}
	if freq, _ := s.ChangeFrequency(ctx, proj.ID, 1); len(freq) != 1 {
		t.Errorf("limit not applied: %+v", freq)
	}

	if err := s.AddAudit(ctx, &AuditEvent{ID: id("audit"), OrgID: orgA.ID, ActorID: userA.ID, Action: "project.create", Resource: "project", ResourceID: proj.ID}); err != nil {
		t.Errorf("AddAudit: %v", err)
	}
	if err := s.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
