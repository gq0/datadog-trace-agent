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

func getTestServiceSampler() *ServiceSampler {
	// Disable debug logs in these tests
	log.UseLogger(log.Disabled)

	// No extra fixed sampling, no maximum TPS
	extraRate := 1.0
	maxTPS := 0.0

	return NewServiceSampler(extraRate, maxTPS, NewRateByService(time.Hour))
}

func getTestTraceWithService(t *testing.T, service string, rates *RateByService) (model.Trace, *model.Span) {
	tID := randomTraceID()
	trace := model.Trace{
		model.Span{TraceID: tID, SpanID: 1, ParentID: 0, Start: 42, Duration: 1000000, Service: service, Type: "web", Meta: map[string]string{"env": defaultEnv}},
		model.Span{TraceID: tID, SpanID: 2, ParentID: 1, Start: 100, Duration: 200000, Service: service, Type: "sql"},
	}
	r := rand.Float64()
	if r <= rates.Get(service, defaultEnv) {
		trace[0].Meta[samplingPriorityKey] = "1"
	}
	return trace, &trace[0]
}

func TestServiceSamplerLoop(t *testing.T) {
	s := getTestServiceSampler()

	exit := make(chan bool)

	go func() {
		s.Run()
		close(exit)
	}()

	s.Stop()

	select {
	case <-exit:
		return
	case <-time.After(time.Second * 1):
		assert.Fail(t, "Sampler took more than 1 second to close")
	}
}

func TestMaxTPSByService(t *testing.T) {
	// Test the "effectiveness" of the maxTPS option.
	assert := assert.New(t)
	s := getTestServiceSampler()

	maxTPS := 5.0
	tps := 100.0
	// To avoid the edge effects from an non-initialized sampler, wait a bit before counting samples.
	initPeriods := 20
	periods := 50

	s.sampler.maxTPS = maxTPS
	periodSeconds := s.sampler.Backend.decayPeriod.Seconds()
	tracesPerPeriod := tps * periodSeconds
	// Set signature score offset high enough not to kick in during the test.
	s.sampler.signatureScoreOffset = 2 * tps
	s.sampler.signatureScoreFactor = math.Pow(s.sampler.signatureScoreSlope, math.Log10(s.sampler.signatureScoreOffset))

	sampledCount := 0
	handledCount := 0

	for period := 0; period < initPeriods+periods; period++ {
		s.sampler.Backend.DecayScore()
		s.sampler.AdjustScoring()
		for i := 0; i < int(tracesPerPeriod); i++ {
			trace, root := getTestTraceWithService(t, "service-a", s.Rates)
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

	// Check that the sampled score pre-maxTPS is equals to the incoming number of traces per second
	//assert.InEpsilon(tps, s.sampler.Backend.GetSampledScore(), 0.01) // [TODO:christian] find out exactly why this is different

	// We should have kept less traces per second than maxTPS
	//assert.InEpsilon(s.sampler.maxTPS, float64(sampledCount)/(float64(handledCount)*periodSeconds), 0.1) // [TODO:christian] find out exactly why this is different

	// We should have a throughput of sampled traces around maxTPS
	// Check for 1% epsilon, but the precision also depends on the backend imprecision (error factor = decayFactor).
	// Combine error rates with L1-norm instead of L2-norm by laziness, still good enough for tests.
	assert.InEpsilon(s.sampler.maxTPS, float64(sampledCount)/(float64(periods)*periodSeconds),
		0.01+s.sampler.Backend.decayFactor-1)
}

// Ensure ServiceSampler implements engine.
var testServiceSampler Engine = &ServiceSampler{}
