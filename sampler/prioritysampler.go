// Package sampler contains all the logic of the agent-side trace sampling
//
// Currently implementation is based on the scoring of the "signature" of each trace
// Based on the score, we get a sample rate to apply to the given trace
//
// Current score implementation is super-simple, it is a counter with polynomial decay per signature.
// We increment it for each incoming trace then we periodically divide the score by two every X seconds.
// Right after the division, the score is an approximation of the number of received signatures over X seconds.
// It is different from the scoring in the Agent.
//
// Since the sampling can happen at different levels (client, agent, server) or depending on different rules,
// we have to track the sample rate applied at previous steps. This way, sampling twice at 50% can result in an
// effective 25% sampling. The rate is stored as a metric in the trace root.
package sampler

import (
	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	samplingPriorityKey = "_sampling_priority_v1"
)

// PrioritySampler is the main component of the sampling logic
type PrioritySampler struct {
	sampler *coreSampler
	rates   *RateByService
}

// NewPrioritySampler returns an initialized Sampler
func NewPrioritySampler(extraRate float64, maxTPS float64, rates *RateByService) *PrioritySampler {
	s := &PrioritySampler{
		sampler: newCoreSampler(extraRate, maxTPS),
		rates:   rates,
	}

	return s
}

// Run runs and block on the Sampler main loop
func (s *PrioritySampler) Run() {
	s.sampler.Run()
}

// Stop stops the main Run loop
func (s *PrioritySampler) Stop() {
	s.sampler.Stop()
}

func updateSampleRateForPriority(root *model.Span, sampleRate float64, rates *RateByService) {
	initialRate := GetTraceAppliedSampleRate(root)
	newRate := initialRate * sampleRate
	SetTraceAppliedSampleRate(root, newRate)

	var env string
	if root.Meta != nil {
		env = root.Meta["env"]
	}
	rates.Set(root.Service, env, newRate)
}

// Sample counts an incoming trace and tells if it is a sample which has to be kept
func (s *PrioritySampler) Sample(trace model.Trace, root *model.Span, env string) bool {
	// Extra safety, just in case one trace is empty
	if len(trace) == 0 {
		return false
	}

	// Regardless of rates, sampling here is based on the metadata set
	// by the client library. Which, is turn, is based on agent hints,
	// but the rule of thumb is: respect client choice.
	sampled := root.Metrics[samplingPriorityKey] > 0

	signature := computeServiceSignature(root, env)

	// Update sampler state by counting this trace
	s.sampler.Backend.CountSignature(signature)

	sampleRate := s.sampler.GetSampleRate(trace, root, signature)
	updateSampleRateForPriority(root, sampleRate, s.rates)

	if sampled {
		// Count the trace to allow us to check for the maxTPS limit.
		// It has to happen before the maxTPS sampling.
		s.sampler.Backend.CountSample()

		// Check for the maxTPS limit, and if we require an extra sampling.
		// No need to check if we already decided not to keep the trace.
		maxTPSrate := s.sampler.GetMaxTPSSampleRate()
		if maxTPSrate < 1 {
			updateSampleRateForPriority(root, sampleRate, s.rates)
		}
	}

	return sampled
}

// GetState collects and return internal statistics and coefficients for indication purposes
// It returns an interface{}, as other samplers might return other informations.
func (s *PrioritySampler) GetState() interface{} {
	return s.sampler.GetState()
}
