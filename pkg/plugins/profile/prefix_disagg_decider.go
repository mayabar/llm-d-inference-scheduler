// Package profile provides profile handler plugin for the epp.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/datalayer/plugins/approximateprefix"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/multi/prefix"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
)

// PrefixDeciderType name of the prefix decider
const PrefixDeciderType = "prefix-disaggregation-decider"

// compile-time type assertion
var _ pdDecider = &PrefixDisaggregationDecider{}
var _ plugins.ConsumerPlugin = &PrefixDisaggregationDecider{}

type prefixDisaggregationDeciderParameters struct {
	// NonCachedTokens non cached tokens limit that triggers disaggregated PD
	NonCachedTokens int `json:"nonCachedTokens"`
	// PrefixPluginName prefix plugin name, optional, should be defined if prefix plugin name is not a default
	PrefixPluginName string `json:"prefixPluginName"`
}

var defaultParams = prefixDisaggregationDeciderParameters{
	NonCachedTokens:  0,
	PrefixPluginName: prefix.PrefixCachePluginType,
}

func (p prefixDisaggregationDeciderParameters) validate() error {
	if p.PrefixPluginName == "" {
		return errors.New("prefixPluginName parameter of prefix disaggregation decider cannot be empty string")
	}

	if p.NonCachedTokens < 0 {
		return errors.New("nonCachedTokens parameter of prefix disaggregation decider cannot be negative")
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
	}, nil
}

// PrefixDisaggregationDecider handles scheduler profiles for PD.
type PrefixDisaggregationDecider struct {
	prefixPluginTypedName plugins.TypedName
	nonCachedTokens       int
}

// isDisaggregationRequired checks if disaggregated PD is required for the given request and pod.
func (d *PrefixDisaggregationDecider) isDisaggregationRequired(ctx context.Context, _ *types.CycleState,
	inputBytesLen int, pod types.Pod) bool {
	if d.nonCachedTokens <= 0 {
		// no disaggregation in case of non cached tokens number is 0
		return false
	}
	if pod == nil {
		log.FromContext(ctx).Info(">> Prefix decider: pod is nil")
		return false
	}
	// inspect the decode pod to decide if prefill should run or not.
	// if the non-cached part is short enough - no disaggregation.
	inputTokensCnt := float64(inputBytesLen) / float64(prefix.AverageCharactersPerToken)
	prefixInfoRaw, ok := pod.Get(approximateprefix.PrefixCacheMatchInfoKey)
	if !ok || prefixInfoRaw == nil {
		log.FromContext(ctx).Info("Unable to read prefix cache state")
		return false
	}
	prefixCacheMatchInfo, ok := prefixInfoRaw.(*approximateprefix.PrefixCacheMatchInfo)
	if !ok {
		log.FromContext(ctx).Info("Wrong type of prefix cache match info")
		return false
	}

	// number of cached tokens
	hitPrefix := float64(prefixCacheMatchInfo.MatchLength())
	// length of the cached part in percentages
	hitPercentagePrefix := hitPrefix / inputTokensCnt
	log.FromContext(ctx).Info("Computed hit percentage for prefix cache",
		"hitPercentage", hitPercentagePrefix, "absolute hit prefix len (tokens)", hitPrefix,
		"prompt length (token)", inputTokensCnt)

	if (1.0-hitPercentagePrefix)*inputTokensCnt < float64(d.nonCachedTokens) {
		log.FromContext(ctx).Info("Non-cached suffix is smaller than threshold, using decode profile only",
			"hitPercentage", hitPercentagePrefix)
		return false // do not run prefill
	}

	return true
}

func (d *PrefixDisaggregationDecider) Consumes() map[string]any {
	return map[string]any{approximateprefix.PrefixCacheMatchInfoKey: approximateprefix.PrefixCacheMatchInfo{}}
}

func (d *PrefixDisaggregationDecider) TypedName() plugins.TypedName {
	return plugins.TypedName{Type: PrefixDeciderType}
}
