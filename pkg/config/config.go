// Package config provides the configuration reading abilities
// Current version read configuration from environment variables
package config

import (
	"encoding/json"
	"math"

	"github.com/go-logr/logr"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/env"
)

const (
	// For every plugin named below, there are four environment variables. They are:
	//  - "ENABLE_" + pluginName  Enables the named plugin for decode processing
	//  - pluginName + "_WEIGHT"  The weight for a scorer in decode processing
	//  - "PREFILL_ENABLE_" + pluginName  Enables the named plugin for prefill processing
	//  - "PREFILL_" + pluginName + "_WEIGHT"  The weight for a scorer in prefill processing

	// KVCacheScorerName name of the kv-cache scorer in configuration
	KVCacheScorerName = "KVCACHE_AWARE_SCORER"
	// LoadAwareScorerName name of the load aware scorer in configuration
	LoadAwareScorerName = "LOAD_AWARE_SCORER"
	// PrefixScorerName name of the prefix scorer in configuration
	PrefixScorerName = "PREFIX_AWARE_SCORER"
	// SessionAwareScorerName name of the session aware scorer in configuration
	SessionAwareScorerName = "SESSION_AWARE_SCORER"

	prefillPrefix = "PREFILL_"
	enablePrefix  = "ENABLE_"
	weightSuffix  = "_WEIGHT"

	// Plugins from Upstream

	// GIELeastKVCacheFilterName name of the GIE least kv-cache filter in configuration
	GIELeastKVCacheFilterName = "GIE_LEAST_KVCACHE_FILTER"
	// GIELeastQueueFilterName name of the GIE least queue filter in configuration
	GIELeastQueueFilterName = "GIE_LEAST_QUEUE_FILTER"
	// GIELoraAffinityFilterName name of the GIE LoRA affinity filter in configuration
	GIELoraAffinityFilterName = "GIE_LORA_AFFINITY_FILTER"
	// GIELowQueueFilterName name of the GIE low queue filter in configuration
	GIELowQueueFilterName = "GIE_LOW_QUEUE_FILTER"
	// GIESheddableCapacityFilterName name of the GIE sheddable capacity filter in configuration
	GIESheddableCapacityFilterName = "GIE_SHEDDABLE_CAPACITY_FILTER"
	// GIEKVCacheUtilizationScorerName name of the GIE kv-cache utilization scorer in configuration
	GIEKVCacheUtilizationScorerName = "GIE_KVCACHE_UTILIZATION_SCORER"
	// GIEQueueScorerName name of the GIE queue scorer in configuration
	GIEQueueScorerName = "GIE_QUEUE_SCORER"
	// GIEPrefixScorerName name of the GIE prefix plugin in configuration
	GIEPrefixScorerName = "GIE_PREFIX_SCORER"

	pdEnabledEnvKey             = "PD_ENABLED"
	pdPromptLenThresholdEnvKey  = "PD_PROMPT_LEN_THRESHOLD"
	pdPromptLenThresholdDefault = 100

	prefixScorerBlockSizeEnvKey  = "PREFIX_SCORER_BLOCK_SIZE"
	prefixScorerBlockSizeDefault = 256

	externalPrefix = "EXTERNAL_"
	httpPrefix     = "HTTP_"

	preSchedulers  = "PRE_SCHEDULERS"
	filters        = "FILTERS"
	scorers        = "SCORERS"
	postSchedulers = "POST_SCHEDULERS"

	// EXTERNAL_HTTP_PRE_SCHEDULERS
	// EXTERNAL_PREFILL_HTTP_PRE_SCHEDULERS
	// EXTERNAL_HTTP_FILTERS
	// EXTERNAL_PREFILL_HTTP_FILTERS
	// EXTERNAL_HTTP_SCORERS
	// EXTERNAL_PREFILL_HTTP_SCORERS
	// EXTERNAL_HTTP_POST_SCHEDULERS
	// EXTERNAL_PREFILL_HTTP_POST_SCHEDULERS
)

type ExternalPluginInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

type ExternalPlugins struct {
	PreSchedulers  []ExternalPluginInfo
	Filters        []ExternalPluginInfo
	Scorers        []ExternalPluginInfo
	PostSchedulers []ExternalPluginInfo
}

// Config contains scheduler configuration, currently configuration is loaded from environment variables
type Config struct {
	logger                          logr.Logger
	DecodeSchedulerPlugins          map[string]int
	PrefillSchedulerPlugins         map[string]int
	DecodeSchedulerExternalPlugins  ExternalPlugins
	PrefillSchedulerExternalPlugins ExternalPlugins

	PDEnabled       bool
	PDThreshold     int
	PrefixBlockSize int
}

// NewConfig creates a new instance if Config
func NewConfig(logger logr.Logger) *Config {
	return &Config{
		logger:                          logger,
		DecodeSchedulerPlugins:          map[string]int{},
		PrefillSchedulerPlugins:         map[string]int{},
		DecodeSchedulerExternalPlugins:  ExternalPlugins{},
		PrefillSchedulerExternalPlugins: ExternalPlugins{},
		PDEnabled:                       false,
		PDThreshold:                     math.MaxInt,
		PrefixBlockSize:                 prefixScorerBlockSizeDefault,
	}
}

// LoadConfig loads configuration from environment variables
func (c *Config) LoadConfig() {
	c.loadPluginInfo(c.DecodeSchedulerPlugins, false,
		KVCacheScorerName, LoadAwareScorerName, PrefixScorerName, SessionAwareScorerName,
		GIELeastKVCacheFilterName, GIELeastQueueFilterName, GIELoraAffinityFilterName,
		GIELowQueueFilterName, GIESheddableCapacityFilterName,
		GIEKVCacheUtilizationScorerName, GIEQueueScorerName, GIEPrefixScorerName)

	c.loadPluginInfo(c.PrefillSchedulerPlugins, true,
		KVCacheScorerName, LoadAwareScorerName, PrefixScorerName, SessionAwareScorerName,
		GIELeastKVCacheFilterName, GIELeastQueueFilterName, GIELoraAffinityFilterName,
		GIELowQueueFilterName, GIESheddableCapacityFilterName,
		GIEKVCacheUtilizationScorerName, GIEQueueScorerName, GIEPrefixScorerName)

	// load external plugins for decode and prefill schedulers
	c.loadExternalPluginsInfo(httpPrefix, "", preSchedulers)
	c.loadExternalPluginsInfo(httpPrefix, "", filter)
	c.loadExternalPluginsInfo(httpPrefix, "", scorers)
	c.loadExternalPluginsInfo(httpPrefix, "", postSchedulers)
	c.loadExternalPluginsInfo(httpPrefix, prefillPrefix, preSchedulers)
	c.loadExternalPluginsInfo(httpPrefix, prefillPrefix, filter)
	c.loadExternalPluginsInfo(httpPrefix, prefillPrefix, scorers)
	c.loadExternalPluginsInfo(httpPrefix, prefillPrefix, postSchedulers)

	c.PDEnabled = env.GetEnvString(pdEnabledEnvKey, "false", c.logger) == "true"
	c.PDThreshold = env.GetEnvInt(pdPromptLenThresholdEnvKey, pdPromptLenThresholdDefault, c.logger)
	c.PrefixBlockSize = env.GetEnvInt(prefixScorerBlockSizeEnvKey, prefixScorerBlockSizeDefault, c.logger)
}

func (c *Config) loadPluginInfo(plugins map[string]int, prefill bool, pluginNames ...string) {
	for _, pluginName := range pluginNames {
		var enablementKey string
		var weightKey string
		if prefill {
			enablementKey = prefillPrefix + enablePrefix + pluginName
			weightKey = prefillPrefix + pluginName + weightSuffix
		} else {
			enablementKey = enablePrefix + pluginName
			weightKey = pluginName + weightSuffix
		}

		if env.GetEnvString(enablementKey, "false", c.logger) != "true" {
			c.logger.Info("Skipping plugin creation as it is not enabled", "name", pluginName)
		} else {
			weight := env.GetEnvInt(weightKey, 1, c.logger)

			plugins[pluginName] = weight
			c.logger.Info("Initialized plugin", "plugin", pluginName, "weight", weight)
		}
	}
}

// loadExternalPluginsInfo loads configuration of external plugins for the given scheduler type and the given plugins type
func (c *Config) loadExternalPluginsInfo(protocol string, schedulerType string, pluginType string) {
	envVarName := externalPrefix + protocol + schedulerType + pluginType
	envVarRawValue := env.GetEnvString(enablementKey, "", c.logger)

	if envVarRawValue == "" {
		return
	}

	var plugins []ExternalPluginInfo{}
	
	json.Unmarshal([]byte(envVarRawValue), plugins)

	fmt.Printf("Plugins: type=%s, info=%+v\n", pluginType, plugins)
}
