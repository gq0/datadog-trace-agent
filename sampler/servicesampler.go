package sampler

import (
	"strconv"

	"github.com/DataDog/datadog-trace-agent/model"
)

// ServiceSampler is sampler that maintains a per-service sampling rate. Used in distributed tracing.
type ServiceSampler struct {
	// Rates contains the current rates for the service sampler. While the service sampler
	// should be fed by one and only one goroutine, the rates can be queried any time and
	// are thread-safe, so it's safe to share this among different goroutines.
	Rates *RateByService

	sampler *ScoreSampler
}

// NewServiceSampler returns a new service sampler.
func NewServiceSampler(extraRate, maxTps float64, rates *RateByService) *ServiceSampler {
	return &ServiceSampler{
		Rates:   rates,
		sampler: newGenericSampler(extraRate, maxTps, &serviceSignatureComputer{}, &clientSampleRateApplier{rates: rates}),
	}
}

// Sample counts an incoming trace and tells if it is a sample which has to be kept.
func (ss *ServiceSampler) Sample(trace model.Trace, root *model.Span, env string) bool {
	// Pipe the trace through the generic sampler to update the stats, but trust
	// the data set by the client library, which is where the decision is taken.
	_ = ss.sampler.Sample(trace, root, env)
	if samplingPriority, err := strconv.Atoi(root.Meta[samplingPriorityKey]); err == nil {
		return samplingPriority > 0
	}
	return false
}

// Run the sampler.
func (ss *ServiceSampler) Run() {
	ss.sampler.Run()
}

// Stop the sampler.
func (ss *ServiceSampler) Stop() {
	ss.sampler.Stop()
}
