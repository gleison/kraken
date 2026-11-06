package domain

import "fmt"

func (activity *Activity) Validate() error {
	if activity == nil {
		return fmt.Errorf("activity: nil contract")
	}
	if err := activity.validateIdentity(); err != nil {
		return err
	}
	if err := validateContracts(activity.Scope, activity.Feedforward, activity.Budget, activity.Correction); err != nil {
		return err
	}
	if err := activity.validateAcceptanceContract(); err != nil {
		return err
	}
	return validateCapabilities(activity.Capabilities)
}

func (activity *Activity) validateIdentity() error {
	if err := validateIdentifier("activity.id", activity.ID); err != nil {
		return err
	}
	if err := validateRequiredText("activity.title", activity.Title); err != nil {
		return err
	}
	if err := validateRequiredText("activity.goal", activity.Goal); err != nil {
		return err
	}
	if err := validateIdentifier("activity.role", string(activity.Role)); err != nil {
		return err
	}
	for _, dependencyID := range activity.DependsOn {
		if err := validateIdentifier("activity.depends_on", dependencyID); err != nil {
			return err
		}
	}
	return nil
}

func (activity *Activity) validateAcceptanceContract() error {
	criterionIDs, err := validateCriteria(activity.Criteria)
	if err != nil {
		return err
	}
	sensorIDs, err := validateSensors(activity.Sensors, criterionIDs)
	if err != nil {
		return err
	}
	if err := validateSensorCoverage(activity.Criteria, activity.Sensors); err != nil {
		return err
	}
	artifactIDs, err := validateArtifacts(activity.Artifacts)
	if err != nil {
		return err
	}
	return validateEvidence(activity.Evidence, criterionIDs, sensorIDs, artifactIDs)
}

func validateCriteria(criteria []Criterion) (map[string]bool, error) {
	if len(criteria) == 0 {
		return nil, fmt.Errorf("activity.criteria: at least one criterion is required")
	}
	identifiers := make(map[string]bool)
	for _, criterion := range criteria {
		if err := registerUniqueIdentifier(identifiers, "criteria.id", criterion.ID); err != nil {
			return nil, err
		}
		if err := validateRequiredText("criteria.description", criterion.Description); err != nil {
			return nil, err
		}
	}
	return identifiers, nil
}

func validateSensors(sensors []Sensor, criterionIDs map[string]bool) (map[string]bool, error) {
	if len(sensors) == 0 {
		return nil, fmt.Errorf("activity.sensors: at least one sensor is required")
	}
	identifiers := make(map[string]bool)
	for _, sensor := range sensors {
		if err := registerUniqueIdentifier(identifiers, "sensors.id", sensor.ID); err != nil {
			return nil, err
		}
		if err := sensor.Validate(); err != nil {
			return nil, fmt.Errorf("sensor %q: %w", sensor.ID, err)
		}
		if err := validateRequiredReferences("sensor.criterion_ids", sensor.CriterionIDs, criterionIDs); err != nil {
			return nil, err
		}
	}
	return identifiers, nil
}

func validateSensorCoverage(criteria []Criterion, sensors []Sensor) error {
	coveredCriteria := make(map[string]bool)
	for _, sensor := range sensors {
		for _, criterionID := range sensor.CriterionIDs {
			coveredCriteria[criterionID] = true
		}
	}
	for _, criterion := range criteria {
		if !coveredCriteria[criterion.ID] {
			return fmt.Errorf("criteria %q: no sensor declared", criterion.ID)
		}
	}
	return nil
}

func validateArtifacts(artifacts []ExpectedArtifact) (map[string]bool, error) {
	identifiers := make(map[string]bool)
	for _, artifact := range artifacts {
		if err := registerUniqueIdentifier(identifiers, "artifacts.id", artifact.ID); err != nil {
			return nil, err
		}
		if err := validateRequiredText("artifacts.description", artifact.Description); err != nil {
			return nil, err
		}
	}
	return identifiers, nil
}

func validateEvidence(evidence []ExpectedEvidence, criterionIDs, sensorIDs, artifactIDs map[string]bool) error {
	if len(evidence) == 0 {
		return fmt.Errorf("activity.evidence: at least one evidence expectation is required")
	}
	identifiers := make(map[string]bool)
	for _, expectation := range evidence {
		if err := registerUniqueIdentifier(identifiers, "evidence.id", expectation.ID); err != nil {
			return err
		}
		if err := validateRequiredText("evidence.description", expectation.Description); err != nil {
			return err
		}
		if err := validateEvidenceReferences(expectation, criterionIDs, sensorIDs, artifactIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateEvidenceReferences(evidence ExpectedEvidence, criterionIDs, sensorIDs, artifactIDs map[string]bool) error {
	if err := validateRequiredReferences("evidence.criterion_ids", evidence.CriterionIDs, criterionIDs); err != nil {
		return err
	}
	if err := validateRequiredReferences("evidence.sensor_ids", evidence.SensorIDs, sensorIDs); err != nil {
		return err
	}
	return validateReferences("evidence.artifact_ids", evidence.ArtifactIDs, artifactIDs)
}

func validateCapabilities(capabilities []Capability) error {
	for _, capability := range capabilities {
		if err := validateIdentifier("capability.name", capability.Name); err != nil {
			return err
		}
		if err := validateRequiredText("capability.resource", capability.Resource); err != nil {
			return err
		}
	}
	return nil
}
