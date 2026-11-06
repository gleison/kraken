package domain

type Role string

const (
	RoleProduct  Role = "product"
	RoleBackend  Role = "backend"
	RoleFrontend Role = "frontend"
	RoleQA       Role = "qa"
	RoleReviewer Role = "reviewer"
	RolePlatform Role = "platform"
	RoleSecurity Role = "security"
	RoleUX       Role = "ux"
	RoleData     Role = "data"
)

type Scope struct {
	Description  string   `json:"description"`
	AllowedPaths []string `json:"allowed_paths"`
	Resources    []string `json:"resources"`
}

type Feedforward struct {
	Context          string   `json:"context"`
	Decisions        []string `json:"decisions"`
	Constraints      []string `json:"constraints"`
	References       []string `json:"references"`
	DefinitionOfDone []string `json:"definition_of_done"`
}

type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type SensorKind string
type SensorCategory string

const (
	SensorComputational SensorKind = "computational"
	SensorInferential   SensorKind = "inferential"

	SensorBehavior        SensorCategory = "behavior"
	SensorArchitecture    SensorCategory = "architecture"
	SensorMaintainability SensorCategory = "maintainability"
)

type Sensor struct {
	ID           string         `json:"id"`
	Description  string         `json:"description"`
	Kind         SensorKind     `json:"kind"`
	Category     SensorCategory `json:"category"`
	Command      []string       `json:"command,omitempty"`
	Role         Role           `json:"role,omitempty"`
	CriterionIDs []string       `json:"criterion_ids"`
}

type Capability struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
}

type ExpectedArtifact struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	MediaType   string `json:"media_type,omitempty"`
}

type ExpectedEvidence struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	CriterionIDs []string `json:"criterion_ids"`
	SensorIDs    []string `json:"sensor_ids"`
	ArtifactIDs  []string `json:"artifact_ids"`
}

type Budget struct {
	MaxCalls           int   `json:"max_calls"`
	MaxTokens          int64 `json:"max_tokens"`
	MaxDurationSeconds int64 `json:"max_duration_seconds"`
	MaxConcurrency     int   `json:"max_concurrency"`
	MaxStorageBytes    int64 `json:"max_storage_bytes"`
}

type CorrectionPolicy struct {
	MaxAttempts int `json:"max_attempts"`
}

func DefaultCorrectionPolicy() CorrectionPolicy {
	return CorrectionPolicy{MaxAttempts: 3}
}

type Activity struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Goal         string             `json:"goal"`
	DependsOn    []string           `json:"depends_on"`
	Role         Role               `json:"role"`
	Scope        Scope              `json:"scope"`
	Feedforward  Feedforward        `json:"feedforward"`
	Criteria     []Criterion        `json:"criteria"`
	Sensors      []Sensor           `json:"sensors"`
	Capabilities []Capability       `json:"capabilities"`
	Artifacts    []ExpectedArtifact `json:"artifacts"`
	Evidence     []ExpectedEvidence `json:"evidence"`
	Budget       Budget             `json:"budget"`
	Correction   CorrectionPolicy   `json:"correction"`
}
