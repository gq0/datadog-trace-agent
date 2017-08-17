package sampler

import (
	"strconv"

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	samplingPriorityKey = "sampling.priority"
)

// SampleRateApplier is an abstraction defining how a rate should be applied to traces.
type SampleRateApplier interface {
	// ApplySampleRate applies a sample rate over a trace root, returning if the trace should be sampled or not.
	ApplySampleRate(root *model.Span, sampleRate float64) bool
}

type agentSampleRateApplier struct {
}

// ApplySampleRate applies a sample rate over a trace root, returning if the trace should be sampled or not.
// It takes into account any previous sampling.
func (asra *agentSampleRateApplier) ApplySampleRate(root *model.Span, sampleRate float64) bool {
	initialRate := GetTraceAppliedSampleRate(root)
	newRate := initialRate * sampleRate
	SetTraceAppliedSampleRate(root, newRate)

	traceID := root.TraceID

	return SampleByRate(traceID, newRate)
}

type clientSampleRateApplier struct {
	rates *RateByService
}

func (csra *clientSampleRateApplier) ApplySampleRate(root *model.Span, sampleRate float64) bool {
	if root.ParentID == 0 {
		env := root.Meta["env"]                       // caveat: won't work if env is not set on root span
		csra.rates.Set(root.Service, env, sampleRate) // fine as RateByService is thread-safe
	}

	if samplingPriority, ok := root.Meta[samplingPriorityKey]; ok {
		if v, err := strconv.Atoi(samplingPriority); err == nil {
			return v > 0
		}
	}

	return false
}
