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
	"math"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

const (
	// Sampler parameters not (yet?) configurable
	defaultDecayPeriod          time.Duration = 5 * time.Second
	adjustPeriod                time.Duration = 10 * time.Second
	initialSignatureScoreOffset float64       = 1
	minSignatureScoreOffset     float64       = 0.01
	defaultSignatureScoreSlope  float64       = 3
)

// Engine is a common basic interface for sampler engines.
type Engine interface {
	// Run the sampler.
	Run()
	// Stop the sampler.
	Stop()
	// Sample a trace.
	Sample(trace model.Trace, root *model.Span, env string) bool
	// GetState returns information about the sampler.
	GetState() interface{}
}

// ScoreSampler is the main component of the sampling logic
type ScoreSampler struct {
	// Storage of the state of the sampler
	Backend *Backend

	// Extra sampling rate to combine to the existing sampling
	extraRate float64
	// Maximum limit to the total number of traces per second to sample
	maxTPS float64

	// Sample any signature with a score lower than scoreSamplingOffset
	// It is basically the number of similar traces per second after which we start sampling
	signatureScoreOffset float64
	// Logarithm slope for the scoring function
	signatureScoreSlope float64
	// signatureScoreFactor = math.Pow(signatureScoreSlope, math.Log10(scoreSamplingOffset))
	signatureScoreFactor float64

	computer SignatureComputer
	applier  SampleRateApplier

	exit chan struct{}
}

// NewSampler returns an initialized Sampler
func NewSampler(extraRate float64, maxTPS float64) *ScoreSampler {
	return newGenericSampler(extraRate, maxTPS, &combinedSignatureComputer{}, &agentSampleRateApplier{})
}

// newGenericSampler returns an initialized Sampler, allowing to choose the signature computer and the sample rate applier.
func newGenericSampler(extraRate float64, maxTPS float64, computer SignatureComputer, applier SampleRateApplier) *ScoreSampler {
	decayPeriod := defaultDecayPeriod

	s := &ScoreSampler{
		Backend:   NewBackend(decayPeriod),
		extraRate: extraRate,
		maxTPS:    maxTPS,

		computer: computer,
		applier:  applier,

		exit: make(chan struct{}),
	}

	s.SetSignatureCoefficients(initialSignatureScoreOffset, defaultSignatureScoreSlope)

	return s
}

// SetSignatureCoefficients updates the internal scoring coefficients used by the signature scoring
func (s *ScoreSampler) SetSignatureCoefficients(offset float64, slope float64) {
	s.signatureScoreOffset = offset
	s.signatureScoreSlope = slope
	s.signatureScoreFactor = math.Pow(slope, math.Log10(offset))
}

// UpdateExtraRate updates the extra sample rate
func (s *ScoreSampler) UpdateExtraRate(extraRate float64) {
	s.extraRate = extraRate
}

// UpdateMaxTPS updates the max TPS limit
func (s *ScoreSampler) UpdateMaxTPS(maxTPS float64) {
	s.maxTPS = maxTPS
}

// Run runs and block on the Sampler main loop
func (s *ScoreSampler) Run() {
	go func() {
		defer watchdog.LogOnPanic()
		s.Backend.Run()
	}()
	s.RunAdjustScoring()
}

// Stop stops the main Run loop
func (s *ScoreSampler) Stop() {
	s.Backend.Stop()
	close(s.exit)
}

// RunAdjustScoring is the sampler feedback loop to adjust the scoring coefficients
func (s *ScoreSampler) RunAdjustScoring() {
	t := time.NewTicker(adjustPeriod)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			s.AdjustScoring()
		case <-s.exit:
			return
		}
	}
}

// Sample counts an incoming trace and tells if it is a sample which has to be kept
func (s *ScoreSampler) Sample(trace model.Trace, root *model.Span, env string) bool {
	// Extra safety, just in case one trace is empty
	if len(trace) == 0 {
		return false
	}

	signature := s.computer.ComputeSignatureWithRootAndEnv(trace, root, env)

	// Update sampler state by counting this trace
	s.Backend.CountSignature(signature)

	sampleRate := s.GetSampleRate(trace, root, signature)

	sampled := s.applier.ApplySampleRate(root, sampleRate)

	if sampled {
		// Count the trace to allow us to check for the maxTPS limit.
		// It has to happen before the maxTPS sampling.
		s.Backend.CountSample()

		// Check for the maxTPS limit, and if we require an extra sampling.
		// No need to check if we already decided not to keep the trace.
		maxTPSrate := s.GetMaxTPSSampleRate()
		//		if maxTPSrate < 1 {
		if maxTPSrate < sampleRate { // [TODO:christian] double-check this
			sampled = s.applier.ApplySampleRate(root, maxTPSrate)
		}
	}

	return sampled
}

// GetSampleRate returns the sample rate to apply to a trace.
func (s *ScoreSampler) GetSampleRate(trace model.Trace, root *model.Span, signature Signature) float64 {
	sampleRate := s.GetSignatureSampleRate(signature) * s.extraRate

	return sampleRate
}

// GetMaxTPSSampleRate returns an extra sample rate to apply if we are above maxTPS.
func (s *ScoreSampler) GetMaxTPSSampleRate() float64 {
	// When above maxTPS, apply an additional sample rate to statistically respect the limit
	maxTPSrate := 1.0
	if s.maxTPS > 0 {
		currentTPS := s.Backend.GetUpperSampledScore()
		if currentTPS > s.maxTPS {
			maxTPSrate = s.maxTPS / currentTPS
		}
	}

	return maxTPSrate
}

// GetTraceAppliedSampleRate gets the sample rate the sample rate applied earlier in the pipeline.
func GetTraceAppliedSampleRate(root *model.Span) float64 {
	if rate, ok := root.Metrics[model.SpanSampleRateMetricKey]; ok {
		return rate
	}

	return 1.0
}

// SetTraceAppliedSampleRate sets the currently applied sample rate in the trace data to allow chained up sampling.
func SetTraceAppliedSampleRate(root *model.Span, sampleRate float64) {
	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	root.Metrics[model.SpanSampleRateMetricKey] = sampleRate
}
