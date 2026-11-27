package orchestrator

import (
	"beacon-go/internal/intenttree"
	"time"
)

type CaptureCommand struct {
	Adapter       string   `json:"adapter,omitempty"`
	Schema        string   `json:"schema"`
	Argv          []string `json:"argv"`
	TestFiles     []string `json:"test_files"`
	RequiredTests []string `json:"required_tests"`
}

type CaptureRequest struct {
	Root, TreeDir, NodeID, ACID, Phase, By, RedID string
	Timeout                                       time.Duration
}

type CaptureResult = intenttree.CaptureEvidence
