package domain

import "slices"

// ChangeClassification states how an API change affects consumers.
type ChangeClassification string

const (
	Breaking    ChangeClassification = "BREAKING"
	NonBreaking ChangeClassification = "NON_BREAKING"
	Conditional ChangeClassification = "CONDITIONAL"
)

// AllClassifications lists every valid classification.
func AllClassifications() []ChangeClassification {
	return []ChangeClassification{Breaking, NonBreaking, Conditional}
}

// Valid reports whether c is a known classification.
func (c ChangeClassification) Valid() bool { return slices.Contains(AllClassifications(), c) }

// Severity expresses how serious a change is. Higher rank means more severe.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// AllSeverities lists severities from least to most severe.
func AllSeverities() []Severity {
	return []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
}

// Valid reports whether s is a known severity.
func (s Severity) Valid() bool { return slices.Contains(AllSeverities(), s) }

// Rank returns the position of s from least to most severe, or -1 if invalid.
func (s Severity) Rank() int { return slices.Index(AllSeverities(), s) }

// CompatibilityDirection distinguishes backward from forward compatibility.
// Backward: can the new provider serve existing consumers?
// Forward: can new consumers work with the old provider?
type CompatibilityDirection string

const (
	DirectionBackward CompatibilityDirection = "BACKWARD"
	DirectionForward  CompatibilityDirection = "FORWARD"
)

// AllDirections lists every valid direction.
func AllDirections() []CompatibilityDirection {
	return []CompatibilityDirection{DirectionBackward, DirectionForward}
}

// Valid reports whether d is a known direction.
func (d CompatibilityDirection) Valid() bool { return slices.Contains(AllDirections(), d) }

// ChangeType identifies the kind of difference detected between two versions.
type ChangeType string

const (
	ChangeEndpointAdded     ChangeType = "ENDPOINT_ADDED"
	ChangeEndpointRemoved   ChangeType = "ENDPOINT_REMOVED"
	ChangeMethodRemoved     ChangeType = "METHOD_REMOVED"
	ChangeParameterAdded    ChangeType = "PARAMETER_ADDED"
	ChangeParameterRemoved  ChangeType = "PARAMETER_REMOVED"
	ChangeRequiredChanged   ChangeType = "REQUIRED_CHANGED"
	ChangeTypeChanged       ChangeType = "TYPE_CHANGED"
	ChangeFieldRemoved      ChangeType = "FIELD_REMOVED"
	ChangeFieldAdded        ChangeType = "FIELD_ADDED"
	ChangeEnumChanged       ChangeType = "ENUM_CHANGED"
	ChangeStatusCodeRemoved ChangeType = "STATUS_CODE_REMOVED"
	ChangeAuthChanged       ChangeType = "AUTH_CHANGED"
	ChangeStatusCodeAdded   ChangeType = "STATUS_CODE_ADDED"
	ChangeConstraintChanged ChangeType = "CONSTRAINT_CHANGED"
)

// AllChangeTypes lists every valid change type.
func AllChangeTypes() []ChangeType {
	return []ChangeType{
		ChangeEndpointAdded, ChangeEndpointRemoved, ChangeMethodRemoved,
		ChangeParameterAdded, ChangeParameterRemoved, ChangeRequiredChanged,
		ChangeTypeChanged, ChangeFieldRemoved, ChangeFieldAdded,
		ChangeEnumChanged, ChangeStatusCodeRemoved, ChangeAuthChanged,
		ChangeStatusCodeAdded, ChangeConstraintChanged,
	}
}

// Valid reports whether t is a known change type.
func (t ChangeType) Valid() bool { return slices.Contains(AllChangeTypes(), t) }

// TestStatus is the outcome state of a test case.
type TestStatus string

const (
	TestStatusPending TestStatus = "PENDING"
	TestStatusRunning TestStatus = "RUNNING"
	TestStatusPassed  TestStatus = "PASSED"
	TestStatusFailed  TestStatus = "FAILED"
	TestStatusSkipped TestStatus = "SKIPPED"
	TestStatusErrored TestStatus = "ERRORED"
)

// Valid reports whether s is a known test status.
func (s TestStatus) Valid() bool {
	return slices.Contains([]TestStatus{
		TestStatusPending, TestStatusRunning, TestStatusPassed,
		TestStatusFailed, TestStatusSkipped, TestStatusErrored,
	}, s)
}

// TestCategory groups generated tests by purpose.
type TestCategory string

const (
	CategorySchemaValidation    TestCategory = "SCHEMA_VALIDATION"
	CategoryParameterValidation TestCategory = "PARAMETER_VALIDATION"
	CategoryBoundary            TestCategory = "BOUNDARY"
	CategoryNegative            TestCategory = "NEGATIVE"
	CategoryStatus              TestCategory = "STATUS"
	CategoryAuthentication      TestCategory = "AUTHENTICATION"
	CategoryContract            TestCategory = "CONTRACT"
	CategoryWorkflow            TestCategory = "WORKFLOW"
)

// Valid reports whether c is a known test category.
func (c TestCategory) Valid() bool {
	return slices.Contains([]TestCategory{
		CategorySchemaValidation, CategoryParameterValidation, CategoryBoundary,
		CategoryNegative, CategoryStatus, CategoryAuthentication,
		CategoryContract, CategoryWorkflow,
	}, c)
}

// PolicyDecision is the outcome of evaluating a release policy.
type PolicyDecision string

const (
	DecisionPass  PolicyDecision = "PASS"
	DecisionWarn  PolicyDecision = "WARN"
	DecisionBlock PolicyDecision = "BLOCK"
)

// Valid reports whether d is a known decision.
func (d PolicyDecision) Valid() bool {
	return slices.Contains([]PolicyDecision{DecisionPass, DecisionWarn, DecisionBlock}, d)
}

// SpecificationStatus tracks the lifecycle of an uploaded specification.
type SpecificationStatus string

const (
	SpecUploaded SpecificationStatus = "UPLOADED"
	SpecValid    SpecificationStatus = "VALID"
	SpecInvalid  SpecificationStatus = "INVALID"
	SpecArchived SpecificationStatus = "ARCHIVED"
)

// Valid reports whether s is a known specification status.
func (s SpecificationStatus) Valid() bool {
	return slices.Contains([]SpecificationStatus{SpecUploaded, SpecValid, SpecInvalid, SpecArchived}, s)
}
