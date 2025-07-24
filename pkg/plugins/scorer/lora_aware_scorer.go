package scorer

import (
	"context"
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
)

const (
	// LoraAwareScorerType is the type of the LoraAwareScorer
	LoraAwareScorerType = "lora-aware-scorer"
)

// compile-time type assertion
var _ framework.Scorer = &LoraAwareScorer{}

// LoraAwareScorerFactory defines the factory function for the LoraAwareScorer
func LoraAwareScorerFactory(name string, _ json.RawMessage, handle plugins.Handle) (plugins.Plugin, error) {
	return NewLoraAwareScorer(handle.Context()).WithName(name), nil
}

// NewLoraAwareScorer creates a new LoRA adapters based scorer
func NewLoraAwareScorer(_ context.Context) *LoraAwareScorer {
	return &LoraAwareScorer{
		name: LoraAwareScorerType,
	}
}

// LoraAwareScorer scorer that is based on loaded LoRA adapters
type LoraAwareScorer struct {
	name string
}

// Type returns the type of the scorer.
func (s *LoraAwareScorer) Type() string {
	return LoraAwareScorerType
}

// Name returns the name of the instance of the filter.
func (s *LoraAwareScorer) Name() string {
	return s.name
}

// WithName sets the name of the filter.
func (s *LoraAwareScorer) WithName(name string) *LoraAwareScorer {
	s.name = name
	return s
}

// Score scores the given pod in range of 0-1
func (s *LoraAwareScorer) Score(ctx context.Context, _ *types.CycleState, request *types.LLMRequest, pods []types.Pod) map[types.Pod]float64 {
	scoredPods := make(map[types.Pod]float64)
	logger := log.FromContext(ctx).WithName(s.name)

	logger.Info(">>> LORA AWARE SCORER - start <<<")

	for _, pod := range pods {
		logger.Info(">>> Check lora on pod", "lora", request.TargetModel, "pod", pod.GetPod().NamespacedName.String())
		if _, ok := pod.GetMetrics().ActiveModels[request.TargetModel]; ok {
			// lora is running on this pod
			scoredPods[pod] = 1.0
			logger.Info("Lora is running on a pod", "lora", request.TargetModel, "pod", pod.GetPod().NamespacedName.String())
		} else if _, ok := pod.GetMetrics().WaitingModels[request.TargetModel]; ok {
			// lora is waiting on this pod
			scoredPods[pod] = 1.0
			logger.Info("Lora is waiting on a pod", "lora", request.TargetModel, "pod", pod.GetPod().NamespacedName.String())
		} else {
			scoredPods[pod] = 0.0
			logger.Info("Lora is NOT loaded on a pod", "lora", request.TargetModel, "pod", pod.GetPod().NamespacedName.String())
		}
	}

	return scoredPods
}
