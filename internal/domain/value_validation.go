package domain

import (
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (budget Budget) Validate() error {
	if budget.MaxCalls <= 0 || budget.MaxTokens <= 0 || budget.MaxDurationSeconds <= 0 ||
		budget.MaxConcurrency <= 0 || budget.MaxStorageBytes <= 0 {
		return fmt.Errorf("budget: calls, tokens, duration_seconds, concurrency and storage_bytes must all be positive")
	}
	return nil
}

func (policy CorrectionPolicy) Validate() error {
	if policy.MaxAttempts <= 0 {
		return fmt.Errorf("correction.max_attempts: must be positive")
	}
	return nil
}

func (scope Scope) Validate() error {
	if err := validateRequiredText("scope.description", scope.Description); err != nil {
		return err
	}
	if err := validateAllowedPaths(scope.AllowedPaths); err != nil {
		return err
	}
	return validateResourceIdentifiers(scope.Resources)
}

func validateAllowedPaths(allowedPaths []string) error {
	seenPaths := make(map[string]bool)
	for _, workspacePath := range allowedPaths {
		if err := validateWorkspaceRelativePath(workspacePath); err != nil {
			return err
		}
		if seenPaths[workspacePath] {
			return fmt.Errorf("scope.allowed_paths: duplicate path %q", workspacePath)
		}
		seenPaths[workspacePath] = true
	}
	return nil
}

func validateWorkspaceRelativePath(workspacePath string) error {
	if strings.TrimSpace(workspacePath) == "" || !utf8.ValidString(workspacePath) ||
		path.IsAbs(workspacePath) || path.Clean(workspacePath) != workspacePath ||
		workspacePath == ".." || strings.HasPrefix(workspacePath, "../") ||
		strings.ContainsAny(workspacePath, "\\:*?[]") || strings.IndexFunc(workspacePath, unicode.IsControl) >= 0 {
		return fmt.Errorf("scope.allowed_paths: invalid workspace-relative path %q", workspacePath)
	}
	for _, component := range strings.Split(workspacePath, "/") {
		if strings.EqualFold(component, ".git") {
			return fmt.Errorf("scope.allowed_paths: Git metadata is not an activity path: %q", workspacePath)
		}
	}
	return nil
}

func validateResourceIdentifiers(resources []string) error {
	seenResources := make(map[string]bool)
	for _, resourceID := range resources {
		if err := registerUniqueIdentifier(seenResources, "scope.resources", resourceID); err != nil {
			return err
		}
	}
	return nil
}

func (feedforward Feedforward) Validate() error {
	if err := validateRequiredText("feedforward.context", feedforward.Context); err != nil {
		return err
	}
	if len(feedforward.DefinitionOfDone) == 0 {
		return fmt.Errorf("feedforward.definition_of_done: at least one condition is required")
	}
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"decisions", feedforward.Decisions}, {"constraints", feedforward.Constraints},
		{"references", feedforward.References}, {"definition_of_done", feedforward.DefinitionOfDone},
	} {
		if err := validateTextEntries("feedforward."+field.name, field.values); err != nil {
			return err
		}
	}
	return nil
}

func (sensor Sensor) Validate() error {
	if err := validateIdentifier("sensor.id", sensor.ID); err != nil {
		return err
	}
	if err := validateRequiredText("sensor.description", sensor.Description); err != nil {
		return err
	}
	switch sensor.Category {
	case SensorBehavior, SensorArchitecture, SensorMaintainability:
	default:
		return fmt.Errorf("sensor.category: unknown category %q", sensor.Category)
	}
	return sensor.validateExecutionContract()
}

func (sensor Sensor) validateExecutionContract() error {
	switch sensor.Kind {
	case SensorComputational:
		return sensor.validateComputationalCheck()
	case SensorInferential:
		return sensor.validateInferentialCheck()
	default:
		return fmt.Errorf("sensor.kind: unknown kind %q", sensor.Kind)
	}
}

func (sensor Sensor) validateComputationalCheck() error {
	if len(sensor.Command) == 0 || sensor.Role != "" {
		return fmt.Errorf("sensor.command: computational checks require argv and no reviewer role")
	}
	if err := validateRequiredText("sensor.command[0]", sensor.Command[0]); err != nil {
		return err
	}
	for _, argument := range sensor.Command {
		if strings.ContainsRune(argument, 0) || !utf8.ValidString(argument) {
			return fmt.Errorf("sensor.command: arguments must be valid UTF-8 without NUL")
		}
	}
	return nil
}

func (sensor Sensor) validateInferentialCheck() error {
	if err := validateIdentifier("sensor.role", string(sensor.Role)); err != nil {
		return err
	}
	if len(sensor.Command) != 0 {
		return fmt.Errorf("sensor.command: inferential checks cannot declare argv")
	}
	return nil
}
