// Package dto holds the request bodies of the REST API.
package dto

import "github.com/thanhmaitran/compatlab/internal/domain"

type RegisterRequest struct {
	Organization string `json:"organization" binding:"required"`
	Email        string `json:"email" binding:"required"`
	Password     string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CreateUserRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UploadSpecRequest struct {
	VersionLabel string `json:"version_label" binding:"required"`
	Filename     string `json:"filename"`
	Content      string `json:"content" binding:"required"`
}

type AnalyzeRequest struct {
	BaseVersionID   string `json:"base_version_id" binding:"required"`
	TargetVersionID string `json:"target_version_id" binding:"required"`
}

type RegisterContractRequest struct {
	ProjectID string                   `json:"project_id" binding:"required"`
	Contract  *domain.ConsumerContract `json:"contract" binding:"required"`
}

type VerifyContractRequest struct {
	VersionID string `json:"version_id" binding:"required"`
}

type GenerateSuiteRequest struct {
	VersionID           string `json:"version_id" binding:"required"`
	AnalysisID          string `json:"analysis_id"`
	MaxCasesPerEndpoint int    `json:"max_cases_per_endpoint"`
	Seed                int64  `json:"seed"`
}

type RunTestsRequest struct {
	SuiteID   string `json:"suite_id" binding:"required"`
	VersionID string `json:"version_id" binding:"required"`
	Target    string `json:"target"`
}

type CreatePolicyRequest struct {
	ProjectID string                `json:"project_id" binding:"required"`
	Policy    *domain.ReleasePolicy `json:"policy" binding:"required"`
}

type GateRequest struct {
	ProjectID   string   `json:"project_id" binding:"required"`
	AnalysisID  string   `json:"analysis_id" binding:"required"`
	PolicyID    string   `json:"policy_id" binding:"required"`
	ContractIDs []string `json:"contract_ids"`
	TestRunID   string   `json:"test_run_id"`
}
