package specification

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", rel))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestParseValidFixtures(t *testing.T) {
	tests := []struct {
		file      string
		endpoints int
		schemas   int
	}{
		{"valid/simple-api.yaml", 3, 2},
		{"valid/payment-api.yaml", 3, 3},
		{"compatible/users-v1.yaml", 4, 3},
	}
	for _, tc := range tests {
		api, err := NewParser().Parse(readFixture(t, tc.file))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if len(api.Endpoints) != tc.endpoints {
			t.Errorf("%s: %d endpoints, want %d", tc.file, len(api.Endpoints), tc.endpoints)
		}
		if len(api.Schemas) != tc.schemas {
			t.Errorf("%s: %d schemas, want %d", tc.file, len(api.Schemas), tc.schemas)
		}
		if api.SourceHash == "" {
			t.Errorf("%s: missing source hash", tc.file)
		}
	}
}

func TestParseInvalidFixtures(t *testing.T) {
	tests := []struct {
		file string
		want error
	}{
		{"invalid/malformed-yaml.yaml", ErrParse},
		{"invalid/missing-openapi-version.yaml", ErrValidation},
		{"invalid/invalid-schema.yaml", ErrValidation},
	}
	for _, tc := range tests {
		_, err := NewParser().Parse(readFixture(t, tc.file))
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: error = %v, want %v", tc.file, err, tc.want)
		}
	}
}

func TestParsePaymentDetails(t *testing.T) {
	api, err := NewParser().Parse(readFixture(t, "valid/payment-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ep, ok := api.Endpoint("GET /payments/{}")
	if !ok {
		t.Fatal("GET /payments/{paymentId} not found")
	}
	if len(ep.Parameters) != 1 || !ep.Parameters[0].Required || ep.Parameters[0].Name != "paymentId" {
		t.Errorf("parameter $ref not resolved: %+v", ep.Parameters)
	}
	if len(ep.Security) != 1 || ep.Security[0].Name != "bearerAuth" {
		t.Errorf("global security not inherited: %+v", ep.Security)
	}
	refund, ok := api.Endpoint("POST /payments/{}/refunds")
	if !ok || len(refund.Parameters) != 1 {
		t.Fatalf("path-level parameter not merged: %+v", refund)
	}
	money := api.Schemas["Money"]
	if money == nil || len(money.Properties["currency"].Enum) != 3 {
		t.Errorf("enum not extracted: %+v", money)
	}
}

func TestParseCircularReference(t *testing.T) {
	api, err := NewParser().Parse(readFixture(t, "valid/simple-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat := api.Schemas["Category"]
	if cat == nil || cat.Properties["parent"] != cat {
		t.Fatal("circular reference must resolve to the same schema node")
	}
}

func TestStatusCodesFromYAMLIntegerKeys(t *testing.T) {
	api, err := NewParser().Parse(readFixture(t, "valid/simple-api.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ep, _ := api.Endpoint("GET /health")
	if ep == nil || len(ep.Responses) != 1 || ep.Responses[0].StatusCode != "200" {
		t.Fatalf("integer status key not normalized: %+v", ep)
	}
}

func TestSwagger2IsUnsupported(t *testing.T) {
	_, err := NewParser().Parse([]byte(`{"swagger":"2.0","info":{"title":"x","version":"1"}}`))
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want ErrUnsupported", err)
	}
}

func TestExternalReferencesAreRejected(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"x","version":"1"},"paths":{"/a":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"https://example.com/schema.json"}}}}}}}}}`
	_, err := NewParser().Parse([]byte(doc))
	if !errors.Is(err, ErrReference) {
		t.Errorf("error = %v, want ErrReference", err)
	}
}

func TestMissingReferenceTarget(t *testing.T) {
	doc := `{"openapi":"3.0.3","info":{"title":"x","version":"1"},"paths":{"/a":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Nope"}}}}}}}}}`
	_, err := NewParser().Parse([]byte(doc))
	if !errors.Is(err, ErrReference) {
		t.Errorf("error = %v, want ErrReference", err)
	}
}

func TestDocumentSizeLimit(t *testing.T) {
	p := &Parser{MaxBytes: 10}
	_, err := p.Parse(readFixture(t, "valid/simple-api.yaml"))
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("error = %v, want ErrTooLarge", err)
	}
}

func TestEmptyAndScalarDocuments(t *testing.T) {
	for _, doc := range []string{"", "   \n", "just text"} {
		if _, err := NewParser().Parse([]byte(doc)); !errors.Is(err, ErrParse) {
			t.Errorf("%q: error = %v, want ErrParse", doc, err)
		}
	}
}

func TestJSONAndYAMLProduceTheSameModel(t *testing.T) {
	yamlDoc := `
openapi: 3.0.3
info:
  title: T
  version: "1"
paths:
  /a:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [x]
                properties:
                  x:
                    type: integer
`
	jsonDoc := `{"openapi":"3.0.3","info":{"title":"T","version":"1"},"paths":{"/a":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["x"],"properties":{"x":{"type":"integer"}}}}}}}}}}}`
	a, err := NewParser().Parse([]byte(yamlDoc))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewParser().Parse([]byte(jsonDoc))
	if err != nil {
		t.Fatal(err)
	}
	a.SourceHash, b.SourceHash = "", ""
	if !reflect.DeepEqual(a, b) {
		t.Errorf("models differ:\n%+v\n%+v", a, b)
	}
}

func TestSourceLayoutDoesNotChangeModel(t *testing.T) {
	one := `{"openapi":"3.0.3","info":{"title":"T","version":"1"},"paths":{"/b":{"post":{"responses":{"201":{"description":"c"}}}},"/a":{"get":{"parameters":[{"name":"q","in":"query"},{"name":"a","in":"header"}],"responses":{"404":{"description":"n"},"200":{"description":"o"}}}}}}`
	two := `{"info":{"version":"1","title":"T"},"paths":{"/a":{"get":{"responses":{"200":{"description":"o"},"404":{"description":"n"}},"parameters":[{"in":"header","name":"a"},{"in":"query","name":"q"}]}},"/b":{"post":{"responses":{"201":{"description":"c"}}}}},"openapi":"3.0.3"}`
	a, err := NewParser().Parse([]byte(one))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewParser().Parse([]byte(two))
	if err != nil {
		t.Fatal(err)
	}
	a.SourceHash, b.SourceHash = "", ""
	if !reflect.DeepEqual(a, b) {
		t.Errorf("models differ:\n%+v\n%+v", a, b)
	}
}

func TestOpenAPI31NullableTypeArray(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"components":{"schemas":{"N":{"type":["string","null"]}}}}`
	api, err := NewParser().Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	n := api.Schemas["N"]
	if n.Type != "string" || !n.Nullable {
		t.Errorf("3.1 type array not handled: %+v", n)
	}
}
