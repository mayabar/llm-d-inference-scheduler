// Package profile provides profile handler plugin for the epp.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/multi/prefix"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"

	k8stypes "k8s.io/apimachinery/pkg/types"
)

// compile-time type assertion
var _ pdDecider = &PrefixDisaggregationDecider{}

const prefixDeciderName = "prefix-disaggregation-decider"

type prefixDisaggregationDeciderParameters struct {
	NonCachedTokens  int    `json:"non-cached-tokens"`
	PrefixPluginName string `json:"prefix-plugin-name"`
	BlockSize        int    `json:"block-size"`
}

var defaultParams = prefixDisaggregationDeciderParameters{
	NonCachedTokens:  0,
	PrefixPluginName: prefix.PrefixCachePluginType,
	BlockSize:        64,
}

func (p prefixDisaggregationDeciderParameters) validate() error {
	if p.PrefixPluginName == "" {
		return errors.New("prefixPluginName parameter of prefix disaggregation decider cannot be empty string")
	}

	if p.NonCachedTokens < 0 {
		return errors.New("nonCachedTokens parameter of prefix disaggregation decider cannot be negative")
	}

	if p.BlockSize <= 0 {
		return errors.New("blockSize parameter of prefix disaggregation decider should be positive")
	}

	return nil
}

// NewPdProfileHandler initializes a new PdProfileHandler and returns its pointer.
func newPrefixDisaggregationDecider(rawParameters json.RawMessage) (*PrefixDisaggregationDecider, error) {
	parameters := defaultParams

	if rawParameters != nil {
		if err := json.Unmarshal(rawParameters, &parameters); err != nil {
			return nil, fmt.Errorf("failed to parse the parameters of the prefix disaggregation decider. Error: %s", err)
		}
	}

	if err := parameters.validate(); err != nil {
		return nil, err
	}

	return &PrefixDisaggregationDecider{
		prefixPluginTypedName: plugins.TypedName{Type: prefix.PrefixCachePluginType, Name: parameters.PrefixPluginName},
		nonCachedTokens:       parameters.NonCachedTokens,
		blockSize:             parameters.BlockSize,
	}, nil
}

// PrefixDisaggregationDecider handles scheduler profiles for PD.
type PrefixDisaggregationDecider struct {
	prefixPluginTypedName plugins.TypedName
	nonCachedTokens       int
	blockSize             int
}

// isDisaggregationRequired checks if disaggregated PD is required for the given request and pod.
func (d *PrefixDisaggregationDecider) isDisaggregationRequired(ctx context.Context, cycleState *types.CycleState,
	inputBytesLen int, pod k8stypes.NamespacedName) bool {
	if d.nonCachedTokens <= 0 {
		// no disaggregation in case of non cached tokens number is 0
		return false
	}

	// inspect the decode pod to decide if prefill should run or not.
	// if the request is short enough - no disaggregation
	hitPercentagePrefix := 0.0 // default to 0, meaning no prefix cache hit
	prefixState, err := types.ReadCycleStateKey[*prefix.SchedulingContextState](cycleState, plugins.StateKey(d.prefixPluginTypedName.String()))
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to read prefix state")
		return false
	}

	hitPrefix := max(prefixState.PrefixCacheServers[prefix.ServerID(pod)], 0)
	hitPercentagePrefix = float64(hitPrefix*d.blockSize) / float64(inputBytesLen)
	log.FromContext(ctx).V(logutil.DEBUG).Info("Computed hit percentage for prefix cache",
		"hitPercentage", hitPercentagePrefix, "absolute hit prefix len", hitPrefix, "promptLength", inputBytesLen)

	if (1.0-hitPercentagePrefix)*float64(inputBytesLen) < float64(d.nonCachedTokens) {
		log.FromContext(ctx).Info("Non-cached suffix is smaller than threshold, using decode profile only", "hitPercentage", hitPercentagePrefix)
		return false // do not run prefill
	}

	return true
}
