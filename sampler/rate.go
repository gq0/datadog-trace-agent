package sampler

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	samplingPriorityKey = "_sampling_priority_v1"
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

// ApplySampleRate, when using client sampling, works in two steps:
// - store the sample rate in the rates by service map, so that next time a client
//   asks for the sampling rate for such a trace, it gest this result
// - use the information that was in the meta tags ("_sampling_priority_v1") to
//   decide wether this one should be sampled or not.
func (csra *clientSampleRateApplier) ApplySampleRate(root *model.Span, sampleRate float64) bool {
	// In the distributed case, updating the sample rate of the span might lead
	// to wrong results when there's strong variations of the sampling rate and/or
	// the system is trying to reach its stable state. Anyway, stats should be calculated
	// before this code is run, so impact should be minimal.
	initialRate := GetTraceAppliedSampleRate(root)
	newRate := initialRate * sampleRate
	SetTraceAppliedSampleRate(root, newRate)

	if root.ParentID == 0 {
		// We only set the sampling rate for pure root spans, not for local roots
		// part of a bigger trace in distributed tracing. There's no point in doing
		// this because the decision comes from the caller.
		env := root.Meta["env"]                    // caveat: won't work if env is not set on root span
		csra.rates.Set(root.Service, env, newRate) // fine as RateByService is thread-safe
	}

	return root.Metrics[samplingPriorityKey] > 0
}
