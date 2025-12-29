// Package profile provides profile handler plugin for the epp.
package profile

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
)

// compile-time type assertion
var _ pdDecider = &NeverDisaggregationDecider{}

const neverDeciderName = "never-disaggregation-decider"

// NewNeverDisaggregationDecider initializes a new NeverDisaggregationDecider and returns its pointer.
func NewNeverDisaggregationDecider(_ json.RawMessage) (*NeverDisaggregationDecider, error) {
	return &NeverDisaggregationDecider{}, nil
}

// NeverDisaggregationDecider handles scheduler profiles for PD.
type NeverDisaggregationDecider struct {
}

// isDisaggregationRequired checks if disaggregated PD is required for the given request and pod.
func (d *NeverDisaggregationDecider) isDisaggregationRequired(_ context.Context, _ *types.CycleState,
	_ int, _ types.Pod) bool {
	return false
}
