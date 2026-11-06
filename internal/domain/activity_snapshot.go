package domain

import "slices"

func NewActivity(contract Activity) (*Activity, error) {
	if err := contract.Validate(); err != nil {
		return nil, err
	}
	return contract.Clone(), nil
}

// Clone transfers ownership without sharing mutable collections.
func (activity *Activity) Clone() *Activity {
	if activity == nil {
		return nil
	}
	snapshot := *activity
	snapshot.DependsOn = slices.Clone(activity.DependsOn)
	snapshot.Scope = activity.Scope.Clone()
	snapshot.Feedforward = activity.Feedforward.Clone()
	snapshot.Criteria = slices.Clone(activity.Criteria)
	snapshot.Sensors = slices.Clone(activity.Sensors)
	for index := range snapshot.Sensors {
		snapshot.Sensors[index] = activity.Sensors[index].Clone()
	}
	snapshot.Capabilities = slices.Clone(activity.Capabilities)
	snapshot.Artifacts = slices.Clone(activity.Artifacts)
	snapshot.Evidence = slices.Clone(activity.Evidence)
	for index := range snapshot.Evidence {
		snapshot.Evidence[index] = activity.Evidence[index].Clone()
	}
	return &snapshot
}

func (scope Scope) Clone() Scope {
	scope.AllowedPaths = slices.Clone(scope.AllowedPaths)
	scope.Resources = slices.Clone(scope.Resources)
	return scope
}

func (feedforward Feedforward) Clone() Feedforward {
	feedforward.Decisions = slices.Clone(feedforward.Decisions)
	feedforward.Constraints = slices.Clone(feedforward.Constraints)
	feedforward.References = slices.Clone(feedforward.References)
	feedforward.DefinitionOfDone = slices.Clone(feedforward.DefinitionOfDone)
	return feedforward
}

func (sensor Sensor) Clone() Sensor {
	sensor.Command = slices.Clone(sensor.Command)
	sensor.CriterionIDs = slices.Clone(sensor.CriterionIDs)
	return sensor
}

func (evidence ExpectedEvidence) Clone() ExpectedEvidence {
	evidence.CriterionIDs = slices.Clone(evidence.CriterionIDs)
	evidence.SensorIDs = slices.Clone(evidence.SensorIDs)
	evidence.ArtifactIDs = slices.Clone(evidence.ArtifactIDs)
	return evidence
}
