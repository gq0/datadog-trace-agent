package sampler

import (
	"math"
	"math/rand"
	"testing"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

const (
	testServiceA = "service-a"
	testServiceB = "service-b"
)

func getTestPriorityEngine() *PriorityEngine {
	// Disable debug logs in these tests
	log.UseLogger(log.Disabled)

	// No extra fixed sampling, no maximum TPS
	extraRate := 1.0
	maxTPS := 0.0

	return NewPriorityEngine(extraRate, maxTPS, NewRateByService(time.Hour))
}

func getTestTraceWithService(t *testing.T, service string, rates *RateByService) (model.Trace, *model.Span) {
	tID := randomTraceID()
	trace := model.Trace{
		model.Span{TraceID: tID, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000, Service: service, Type: "web", Meta: map[string]string{"env": defaultEnv}},
		model.Span{TraceID: tID, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000, Service: service, Type: "sql"},
	}
	r := rand.Float64()
	if r <= rates.Get(service, defaultEnv) {
		trace[0].Metrics = map[string]float64{samplingPriorityKey: 1}
	}
	return trace, &trace[0]
}

func TestUpdateSampleRateForPriority(t *testing.T) {
	assert := assert.New(t)
	tID := randomTraceID()

	rates := NewRateByService(time.Hour)
	root := model.Span{
		TraceID:  tID,
		SpanID:   1,
		ParentID: 0,
		Start:    123,
		Duration: 100000,
		Service:  "mcnulty",
		Type:     "web",
		Metrics:  map[string]float64{"_sampling_priority_v1": 1},
	}

	updateSampleRateForPriority(&root, 0.4, rates)
	assert.Equal(0.4, root.Metrics["_sample_rate"], "sample rate should be 100%")
	assert.Equal(map[string]float64{"service:,env:": 1, "service:mcnulty,env:": 0.4}, rates.GetAll())

	delete(root.Metrics, "_sampling_priority_v1")
	updateSampleRateForPriority(&root, 0.5, rates)

	assert.Equal(0.2, root.Metrics["_sample_rate"], "sample rate should be 20% (50% of 40%)")
	assert.Equal(map[string]float64{"service:,env:": 1, "service:mcnulty,env:": 0.2}, rates.GetAll())
}

func TestMaxTPSByService(t *testing.T) {
	// Test the "effectiveness" of the maxTPS option.
	assert := assert.New(t)
	s := getTestPriorityEngine()

	maxTPS := 5.0
	tps := 100.0
	// To avoid the edge effects from an non-initialized sampler, wait a bit before counting samples.
	initPeriods := 50
	periods := 200

	s.Sampler.maxTPS = maxTPS
	periodSeconds := s.Sampler.Backend.decayPeriod.Seconds()
	tracesPerPeriod := tps * periodSeconds
	// Set signature score offset high enough not to kick in during the test.
	s.Sampler.signatureScoreOffset = 2 * tps
	s.Sampler.signatureScoreFactor = math.Pow(s.Sampler.signatureScoreSlope, math.Log10(s.Sampler.signatureScoreOffset))

	sampledCount := 0
	handledCount := 0

	for period := 0; period < initPeriods+periods; period++ {
		s.Sampler.Backend.DecayScore()
		s.Sampler.AdjustScoring()
		for i := 0; i < int(tracesPerPeriod); i++ {
			trace, root := getTestTraceWithService(t, "service-a", s.rates)
			sampled := s.Sample(trace, root, defaultEnv)
			// Once we got into the "supposed-to-be" stable "regime", count the samples
			if period > initPeriods {
				handledCount++
				if sampled {
					sampledCount++
				}
			}
		}
	}

	// Check that the sampled score is roughly equal to maxTPS. This is different from
	// the score sampler test as here we run adjustscoring on a regular basis so the converges to maxTPS.
	assert.InEpsilon(maxTPS, s.Sampler.Backend.GetSampledScore(), 0.1)

	// We should have keep the right percentage of traces
	assert.InEpsilon(s.Sampler.maxTPS/tps, float64(sampledCount)/float64(handledCount), 0.1)

	// We should have a throughput of sampled traces around maxTPS
	// Check for 1% epsilon, but the precision also depends on the backend imprecision (error factor = decayFactor).
	// Combine error rates with L1-norm instead of L2-norm by laziness, still good enough for tests.
	assert.InEpsilon(s.Sampler.maxTPS, float64(sampledCount)/(float64(periods)*periodSeconds),
		0.01+s.Sampler.Backend.decayFactor-1)
}

// Ensure PriorityEngine implements engine.
var testPriorityEngine Engine = &PriorityEngine{}
