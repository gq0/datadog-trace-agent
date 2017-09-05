package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-trace-agent/model"
)

func TestAgentApplySampleRate(t *testing.T) {
	assert := assert.New(t)
	tID := randomTraceID()

	asra := &agentSampleRateApplier{}
	root := model.Span{TraceID: tID, SpanID: 1, ParentID: 0, Start: 123, Duration: 100000, Service: "mcnulty", Type: "web"}

	asra.ApplySampleRate(&root, 0.4)
	assert.Equal(0.4, root.Metrics["_sample_rate"], "sample rate should be 40%")

	asra.ApplySampleRate(&root, 0.5)
	assert.Equal(0.2, root.Metrics["_sample_rate"], "sample rate should be 20% (50% of 40%)")
}

func TestClientApplySampleRate(t *testing.T) {
	assert := assert.New(t)
	tID := randomTraceID()

	csra := &clientSampleRateApplier{rates: NewRateByService(time.Hour)}
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

	var sampled bool

	sampled = csra.ApplySampleRate(&root, 0.4)
	assert.True(sampled)
	assert.Equal(0.4, root.Metrics["_sample_rate"], "sample rate should be 100%")
	assert.Equal(map[string]float64{"service:,env:": 1, "service:mcnulty,env:": 0.4}, csra.rates.GetAll())

	delete(root.Metrics, "_sampling_priority_v1")
	sampled = csra.ApplySampleRate(&root, 0.5)
	assert.False(sampled)
	assert.Equal(0.2, root.Metrics["_sample_rate"], "sample rate should be 20% (50% of 40%)")
	assert.Equal(map[string]float64{"service:,env:": 1, "service:mcnulty,env:": 0.5}, csra.rates.GetAll())
}
