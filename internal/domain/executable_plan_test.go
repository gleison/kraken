package domain_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gleison/kraken/internal/domain"
)

func executablePlanFixture() domain.ExecutablePlan {
	budget := domain.Budget{MaxCalls: 20, MaxTokens: 10000, MaxDurationSeconds: 300, MaxConcurrency: 1, MaxStorageBytes: 1048576}
	return domain.ExecutablePlan{
		ID: "plan-1", Goal: "Implement the approved feature", Budget: budget,
		Activities: []domain.Activity{{
			ID: "A-1", Title: "Implement feature", Goal: "Satisfy the approved example", Role: domain.RoleBackend,
			DependsOn: []string{"docs"},
			Scope:     domain.Scope{Description: "Only feature files", AllowedPaths: []string{"internal/feature"}, Resources: []string{"feature-worktree"}},
			Feedforward: domain.Feedforward{
				Context: "The example in the specification is approved", Decisions: []string{"Use Go"},
				Constraints: []string{"No network"}, References: []string{"docs/spec.md"},
				DefinitionOfDone: []string{"Approved example passes"},
			},
			Criteria: []domain.Criterion{{ID: "C-1", Description: "Example returns the specified result"}},
			Sensors: []domain.Sensor{
				{ID: "tests", Description: "Run example tests", Kind: domain.SensorComputational, Category: domain.SensorBehavior,
					Command: []string{"go", "test", "./internal/feature"}, CriterionIDs: []string{"C-1"}},
				{ID: "review", Description: "Review boundaries", Kind: domain.SensorInferential, Category: domain.SensorArchitecture,
					Role: domain.RoleReviewer, CriterionIDs: []string{"C-1"}},
				{ID: "vet", Description: "Check maintainability", Kind: domain.SensorComputational, Category: domain.SensorMaintainability,
					Command: []string{"go", "vet", "./internal/feature"}, CriterionIDs: []string{"C-1"}},
			},
			Capabilities: []domain.Capability{{Name: "write-worktree", Resource: "internal/feature"}},
			Artifacts:    []domain.ExpectedArtifact{{ID: "report", Description: "Test report", MediaType: "text/plain"}},
			Evidence:     []domain.ExpectedEvidence{{ID: "E-1", Description: "Captured test result", CriterionIDs: []string{"C-1"}, SensorIDs: []string{"tests"}, ArtifactIDs: []string{"report"}}},
			Budget:       budget, Correction: domain.DefaultCorrectionPolicy(),
		}},
	}
}

func TestExecutableContractConstruction(t *testing.T) {
	spec := executablePlanFixture()
	p, err := domain.NewExecutablePlan(spec)
	if err != nil || !reflect.DeepEqual(p, &spec) {
		t.Fatalf("plan=%#v err=%v", p, err)
	}
	a, err := domain.NewActivity(spec.Activities[0])
	if err != nil || !reflect.DeepEqual(a, &spec.Activities[0]) {
		t.Fatalf("activity=%#v err=%v", a, err)
	}
	if a.Correction.MaxAttempts != 3 {
		t.Fatal("default correction policy must allow three attempts")
	}
	a.Scope.AllowedPaths, a.Scope.Resources, a.Capabilities, a.Artifacts = nil, nil, nil, nil
	a.Evidence[0].ArtifactIDs = nil
	a.Role = "custom-reviewer"
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestActivityRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*domain.Activity)
		field string
	}{
		{"missing id", func(a *domain.Activity) { a.ID = "" }, "activity.id"},
		{"invalid id", func(a *domain.Activity) { a.ID = "a/b" }, "activity.id"},
		{"blank title", func(a *domain.Activity) { a.Title = " \t" }, "activity.title"},
		{"blank goal", func(a *domain.Activity) { a.Goal = "" }, "activity.goal"},
		{"nul text", func(a *domain.Activity) { a.Goal = "goal\x00" }, "activity.goal"},
		{"invalid utf8 text", func(a *domain.Activity) { a.Goal = "goal\xff" }, "activity.goal"},
		{"invalid utf8 id", func(a *domain.Activity) { a.ID = "id\xff" }, "activity.id"},
		{"missing role", func(a *domain.Activity) { a.Role = "" }, "activity.role"},
		{"invalid role", func(a *domain.Activity) { a.Role = "bad role" }, "activity.role"},
		{"empty dependency", func(a *domain.Activity) { a.DependsOn = []string{""} }, "depends_on"},
		{"blank scope", func(a *domain.Activity) { a.Scope.Description = "" }, "scope.description"},
		{"duplicate resource", func(a *domain.Activity) { a.Scope.Resources = []string{"r", "r"} }, "scope.resources"},
		{"blank resource", func(a *domain.Activity) { a.Scope.Resources = []string{""} }, "scope.resources"},
		{"missing context", func(a *domain.Activity) { a.Feedforward.Context = "" }, "feedforward.context"},
		{"missing done", func(a *domain.Activity) { a.Feedforward.DefinitionOfDone = nil }, "definition_of_done"},
		{"empty done", func(a *domain.Activity) { a.Feedforward.DefinitionOfDone[0] = "" }, "definition_of_done"},
		{"empty decision", func(a *domain.Activity) { a.Feedforward.Decisions[0] = "" }, "feedforward.decisions"},
		{"empty constraint", func(a *domain.Activity) { a.Feedforward.Constraints[0] = "" }, "feedforward.constraints"},
		{"empty reference", func(a *domain.Activity) { a.Feedforward.References[0] = "" }, "feedforward.references"},
		{"no criteria", func(a *domain.Activity) { a.Criteria = nil }, "criteria"},
		{"blank criterion", func(a *domain.Activity) { a.Criteria[0].Description = "" }, "criteria.description"},
		{"duplicate criterion", func(a *domain.Activity) { a.Criteria = append(a.Criteria, a.Criteria[0]) }, "criteria.id"},
		{"uncovered criterion", func(a *domain.Activity) {
			a.Criteria = append(a.Criteria, domain.Criterion{ID: "C-2", Description: "Another requirement"})
		}, "C-2"},
		{"no sensors", func(a *domain.Activity) { a.Sensors = nil }, "sensors"},
		{"duplicate sensor", func(a *domain.Activity) { a.Sensors = append(a.Sensors, a.Sensors[0]) }, "sensors.id"},
		{"blank sensor", func(a *domain.Activity) { a.Sensors[0].Description = "" }, "sensor.description"},
		{"unknown kind", func(a *domain.Activity) { a.Sensors[0].Kind = "magic" }, "sensor.kind"},
		{"unknown category", func(a *domain.Activity) { a.Sensors[0].Category = "quality" }, "sensor.category"},
		{"missing argv", func(a *domain.Activity) { a.Sensors[0].Command = nil }, "sensor.command"},
		{"blank executable", func(a *domain.Activity) { a.Sensors[0].Command[0] = " " }, "sensor.command"},
		{"nul argv", func(a *domain.Activity) { a.Sensors[0].Command[1] = "test\x00" }, "sensor.command"},
		{"invalid utf8 argv", func(a *domain.Activity) { a.Sensors[0].Command[1] = "test\xff" }, "sensor.command"},
		{"command with role", func(a *domain.Activity) { a.Sensors[0].Role = domain.RoleQA }, "sensor.command"},
		{"inference without role", func(a *domain.Activity) { a.Sensors[1].Role = "" }, "sensor.role"},
		{"inference with argv", func(a *domain.Activity) { a.Sensors[1].Command = []string{"go"} }, "sensor.command"},
		{"sensor without criteria", func(a *domain.Activity) { a.Sensors[0].CriterionIDs = nil }, "sensor.criterion_ids"},
		{"unknown sensor criterion", func(a *domain.Activity) { a.Sensors[0].CriterionIDs = []string{"missing"} }, "sensor.criterion_ids"},
		{"duplicate sensor criterion", func(a *domain.Activity) { a.Sensors[0].CriterionIDs = []string{"C-1", "C-1"} }, "sensor.criterion_ids"},
		{"blank capability", func(a *domain.Activity) { a.Capabilities[0].Name = "" }, "capability.name"},
		{"unscoped capability", func(a *domain.Activity) { a.Capabilities[0].Resource = "" }, "capability.resource"},
		{"blank artifact", func(a *domain.Activity) { a.Artifacts[0].Description = "" }, "artifacts.description"},
		{"duplicate artifact", func(a *domain.Activity) { a.Artifacts = append(a.Artifacts, a.Artifacts[0]) }, "artifacts.id"},
		{"no evidence", func(a *domain.Activity) { a.Evidence = nil }, "evidence"},
		{"duplicate evidence", func(a *domain.Activity) { a.Evidence = append(a.Evidence, a.Evidence[0]) }, "evidence.id"},
		{"blank evidence", func(a *domain.Activity) { a.Evidence[0].Description = "" }, "evidence.description"},
		{"evidence without criterion", func(a *domain.Activity) { a.Evidence[0].CriterionIDs = nil }, "evidence.criterion_ids"},
		{"unknown evidence criterion", func(a *domain.Activity) { a.Evidence[0].CriterionIDs[0] = "missing" }, "evidence.criterion_ids"},
		{"evidence without sensor", func(a *domain.Activity) { a.Evidence[0].SensorIDs = nil }, "evidence.sensor_ids"},
		{"unknown evidence sensor", func(a *domain.Activity) { a.Evidence[0].SensorIDs[0] = "missing" }, "evidence.sensor_ids"},
		{"unknown evidence artifact", func(a *domain.Activity) { a.Evidence[0].ArtifactIDs[0] = "missing" }, "evidence.artifact_ids"},
		{"no budget", func(a *domain.Activity) { a.Budget = domain.Budget{} }, "budget"},
		{"no attempts", func(a *domain.Activity) { a.Correction.MaxAttempts = 0 }, "correction.max_attempts"},
		{"negative attempts", func(a *domain.Activity) { a.Correction.MaxAttempts = -1 }, "correction.max_attempts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := executablePlanFixture().Activities[0]
			tt.edit(&a)
			got, err := domain.NewActivity(a)
			if err == nil || got != nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s error; got=%#v err=%v", tt.field, got, err)
			}
		})
	}
}

func TestScopePaths(t *testing.T) {
	for _, p := range []string{"", " ", "../file", "/tmp/file", "a/../../file", "a/../file", "a//file", "a/", "C:/file", `a\file`, "*.go", "a/[x]", ".git/config", "nested/.GIT/index", "bad\x00file", "bad\nfile", "bad\xfffile"} {
		t.Run(p, func(t *testing.T) {
			s := domain.Scope{Description: "Scoped", AllowedPaths: []string{p}}
			if s.Validate() == nil {
				t.Errorf("accepted invalid path %q", p)
			}
		})
	}
	s := domain.Scope{Description: "Scoped", AllowedPaths: []string{".", "internal/domain", "space dir/é.go"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.AllowedPaths = []string{"file", "file"}
	if s.Validate() == nil {
		t.Fatal("accepted duplicate path")
	}
}

func TestBudgetBounds(t *testing.T) {
	fields := []func(*domain.Budget, int64){
		func(b *domain.Budget, n int64) { b.MaxCalls = int(n) },
		func(b *domain.Budget, n int64) { b.MaxTokens = n },
		func(b *domain.Budget, n int64) { b.MaxDurationSeconds = n },
		func(b *domain.Budget, n int64) { b.MaxConcurrency = int(n) },
		func(b *domain.Budget, n int64) { b.MaxStorageBytes = n },
	}
	for i, edit := range fields {
		for _, invalid := range []int64{0, -1} {
			b := executablePlanFixture().Budget
			edit(&b, invalid)
			if b.Validate() == nil {
				t.Errorf("budget dimension %d accepted %d", i, invalid)
			}
		}
	}
}

func TestPlanRejectsInvalidFields(t *testing.T) {
	for _, edit := range []func(*domain.ExecutablePlan){
		func(p *domain.ExecutablePlan) { p.ID = "" },
		func(p *domain.ExecutablePlan) { p.Goal = " " },
		func(p *domain.ExecutablePlan) { p.Activities = nil },
		func(p *domain.ExecutablePlan) { p.Budget.MaxCalls = 0 },
		func(p *domain.ExecutablePlan) { p.Activities[0].Goal = "" },
	} {
		p := executablePlanFixture()
		edit(&p)
		if got, err := domain.NewExecutablePlan(p); err == nil || got != nil {
			t.Fatalf("invalid plan accepted: %#v", got)
		}
	}
}

func mutateEveryPlanCollection(p *domain.ExecutablePlan) {
	p.ID, p.Goal, p.Budget.MaxCalls = "changed", "changed", 1
	a := &p.Activities[0]
	a.ID = "changed"
	a.DependsOn[0] = "changed"
	a.Scope.AllowedPaths[0], a.Scope.Resources[0] = "changed", "changed"
	a.Feedforward.Context = "changed"
	a.Feedforward.Decisions[0], a.Feedforward.Constraints[0] = "changed", "changed"
	a.Feedforward.References[0], a.Feedforward.DefinitionOfDone[0] = "changed", "changed"
	a.Criteria[0].Description = "changed"
	a.Sensors[0].Description, a.Sensors[0].Command[0], a.Sensors[0].CriterionIDs[0] = "changed", "changed", "changed"
	a.Capabilities[0].Resource, a.Artifacts[0].Description = "changed", "changed"
	a.Evidence[0].Description = "changed"
	a.Evidence[0].CriterionIDs[0], a.Evidence[0].SensorIDs[0], a.Evidence[0].ArtifactIDs[0] = "changed", "changed", "changed"
	a.Budget.MaxCalls, a.Correction.MaxAttempts = 1, 1
}

func TestSnapshotsAndConstructorsOwnAllCollections(t *testing.T) {
	want := executablePlanFixture()
	source := executablePlanFixture()
	clone := source.Clone()
	constructed, err := domain.NewExecutablePlan(source)
	if err != nil {
		t.Fatal(err)
	}
	activity, err := domain.NewActivity(source.Activities[0])
	if err != nil {
		t.Fatal(err)
	}
	mutateEveryPlanCollection(&source)
	for _, p := range []*domain.ExecutablePlan{clone, constructed} {
		if !reflect.DeepEqual(p, &want) {
			t.Fatal("snapshot aliases input")
		}
	}
	if !reflect.DeepEqual(activity, &want.Activities[0]) {
		t.Fatal("activity constructor aliases input")
	}
	mutateEveryPlanCollection(clone)
	if !reflect.DeepEqual(constructed, &want) {
		t.Fatal("separate snapshots alias one another")
	}
	standalone := want.Activities[0].Clone()
	standalone.Sensors[0].Command[0] = "changed"
	if want.Activities[0].Sensors[0].Command[0] != "go" {
		t.Fatal("activity clone aliases sensor command")
	}
}

func TestOwnedSnapshotsCanBeMutatedConcurrently(t *testing.T) {
	p := executablePlanFixture()
	var wg sync.WaitGroup
	for range 8 {
		snapshot := p.Clone()
		wg.Go(func() { mutateEveryPlanCollection(snapshot) })
	}
	wg.Wait()
	if !reflect.DeepEqual(p, executablePlanFixture()) {
		t.Fatal("consumer mutated original")
	}
}

func TestNilContractsAndEmptyClone(t *testing.T) {
	var a *domain.Activity
	var p *domain.ExecutablePlan
	if a.Validate() == nil || p.Validate() == nil || a.Clone() != nil || p.Clone() != nil {
		t.Fatal("nil contracts must fail validation and clone to nil")
	}
	for _, p := range []*domain.ExecutablePlan{{}, {Activities: []domain.Activity{}}} {
		if !reflect.DeepEqual(p, p.Clone()) {
			t.Fatal("clone changed nil/empty collection shape")
		}
	}
}

func TestExecutablePlanJSONRoundTrip(t *testing.T) {
	p := executablePlanFixture()
	p.Activities[0].Sensors[0].Command = []string{"check", "", "space dir", `"quoted"`, "ação"}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"max_duration_seconds":300`) || !strings.Contains(string(encoded), `"activities":`) {
		t.Fatalf("unexpected JSON contract: %s", encoded)
	}
	var restored domain.ExecutablePlan
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, restored) {
		t.Fatal("JSON roundtrip changed the contract")
	}
	again, err := json.Marshal(restored)
	if err != nil || string(again) != string(encoded) {
		t.Fatal("serialization is not stable")
	}
	mutateEveryPlanCollection(&restored)
	if p.Activities[0].Sensors[0].Command[0] != "check" {
		t.Fatal("deserialized plan aliases original")
	}
}

func TestUntrustedJSONNeedsValidation(t *testing.T) {
	var p domain.ExecutablePlan
	if err := json.Unmarshal([]byte(`{"id":"empty-plan","goal":"goal"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.Validate() == nil {
		t.Fatal("incomplete JSON was accepted as an executable contract")
	}
	for _, raw := range []string{
		`{"activities":"wrong type"}`, `{"budget":{"max_calls":1.5}}`,
		`{"budget":{"max_tokens":9223372036854775808}}`, `{"id":`,
	} {
		if json.Unmarshal([]byte(raw), &p) == nil {
			t.Errorf("accepted malformed JSON %s", raw)
		}
	}
}
