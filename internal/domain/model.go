package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ExperimentStatus string

const (
	ExperimentStatusDraft     ExperimentStatus = "draft"
	ExperimentStatusRunning   ExperimentStatus = "running"
	ExperimentStatusPaused    ExperimentStatus = "paused"
	ExperimentStatusCompleted ExperimentStatus = "completed"
)

type PolicyKind string

const (
	PolicyKindRandom                 PolicyKind = "random"
	PolicyKindSegmentedEpsilonGreedy PolicyKind = "segmented_epsilon_greedy"
)

type DeviceClass string

const (
	DeviceClassMobile  DeviceClass = "mobile"
	DeviceClassDesktop DeviceClass = "desktop"
	DeviceClassTablet  DeviceClass = "tablet"
)

type Daypart string

const (
	DaypartMorning   Daypart = "morning"
	DaypartAfternoon Daypart = "afternoon"
	DaypartEvening   Daypart = "evening"
	DaypartNight     Daypart = "night"
)

type OfferCategory string

const (
	OfferCategoryTravel        OfferCategory = "travel"
	OfferCategoryDining        OfferCategory = "dining"
	OfferCategoryWellness      OfferCategory = "wellness"
	OfferCategoryHome          OfferCategory = "home"
	OfferCategoryTechnology    OfferCategory = "technology"
	OfferCategoryEntertainment OfferCategory = "entertainment"
)

type VisitorType string

const (
	VisitorTypeNew       VisitorType = "new"
	VisitorTypeReturning VisitorType = "returning"
)

type OutcomeKind string

const (
	OutcomeKindIgnored   OutcomeKind = "ignored"
	OutcomeKindClicked   OutcomeKind = "clicked"
	OutcomeKindConverted OutcomeKind = "converted"
)

type SimulationRunStatus string

const (
	SimulationRunStatusStarting  SimulationRunStatus = "starting"
	SimulationRunStatusRunning   SimulationRunStatus = "running"
	SimulationRunStatusStopping  SimulationRunStatus = "stopping"
	SimulationRunStatusCompleted SimulationRunStatus = "completed"
	SimulationRunStatusFailed    SimulationRunStatus = "failed"
	SimulationRunStatusCancelled SimulationRunStatus = "cancelled"
)

type BenchmarkKind string

const (
	BenchmarkKindRandom BenchmarkKind = "random"
	BenchmarkKindOracle BenchmarkKind = "oracle"
)

// Experiment defines one immutable policy configuration and its current version.
type Experiment struct {
	ID            uuid.UUID
	Slug          string
	Name          string
	Status        ExperimentStatus
	PolicyKind    PolicyKind
	Epsilon       *float64
	PolicyVersion int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Offer is one fictional action available within an experiment.
type Offer struct {
	ID           uuid.UUID
	ExperimentID uuid.UUID
	Slug         string
	MerchantName string
	Title        string
	Description  string
	Category     OfferCategory
	Active       bool
}

// SessionContext contains the complete privacy-safe policy context persisted as JSON.
type SessionContext struct {
	DeviceClass      DeviceClass   `json:"device_class"`
	Daypart          Daypart       `json:"daypart"`
	CategoryAffinity OfferCategory `json:"category_affinity"`
	VisitorType      VisitorType   `json:"visitor_type"`
}

// ActionProbability records one eligible offer and its exact policy probability.
type ActionProbability struct {
	OfferID     uuid.UUID `json:"offer_id"`
	Probability float64   `json:"probability"`
}

// Decision is the immutable audit record of one policy selection.
type Decision struct {
	ID                  uuid.UUID
	ExperimentID        uuid.UUID
	SelectedOfferID     uuid.UUID
	Context             SessionContext
	SegmentKey          string
	EligibleOfferIDs    []uuid.UUID
	Distribution        []ActionProbability
	Propensity          float64
	PolicyKind          PolicyKind
	PolicyVersion       int64
	PolicyLatencyMicros int64
	SimulationRunID     *uuid.UUID
	RequestID           string
	CreatedAt           time.Time
}

// Outcome is one accepted terminal feedback event and its reserved application version.
type Outcome struct {
	EventID              uuid.UUID
	DecisionID           uuid.UUID
	Kind                 OutcomeKind
	Reward               float64
	OccurredAt           time.Time
	ReceivedAt           time.Time
	AppliedPolicyVersion int64
}

// PolicySnapshot stores one versioned opaque policy state document.
type PolicySnapshot struct {
	ExperimentID  uuid.UUID
	PolicyKind    PolicyKind
	PolicyVersion int64
	SchemaVersion int
	State         json.RawMessage
	CreatedAt     time.Time
}

// SimulationRun records a bounded, reproducible synthetic traffic session.
type SimulationRun struct {
	ID                      uuid.UUID
	ExperimentID            uuid.UUID
	Seed                    int64
	RequestsPerSecond       int
	MaxDecisions            int
	Status                  SimulationRunStatus
	DecisionCount           int64
	OutcomeCount            int64
	ErrorCount              int64
	ObservedRewardSum       float64
	RandomExpectedRewardSum float64
	OracleExpectedRewardSum float64
	StartedAt               time.Time
	StoppedAt               *time.Time
	UpdatedAt               time.Time
	ErrorCode               *string
	ErrorDetail             *string
}

// LearningSeriesPoint is one bounded cumulative-reward projection bucket.
type LearningSeriesPoint struct {
	Timestamp               time.Time
	SampleCount             int64
	CumulativeAverageReward float64
}

// BenchmarkReference is a nullable simulation-only expected-average reference.
type BenchmarkReference struct {
	Kind                  BenchmarkKind
	ExpectedAverageReward *float64
	SampleCount           int64
	Reason                string
	SimulationOnly        bool
}
