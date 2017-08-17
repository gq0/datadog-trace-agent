package sampler

import (
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	// DefaultServiceSamplerTimeout is the time after which we consider we can remove cache data.
	DefaultServiceSamplerTimeout = time.Hour
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
func NewServiceSampler(extraRate, maxTps float64) *ServiceSampler {
	rates := NewRateByService(DefaultServiceSamplerTimeout)
	return &ServiceSampler{
		Rates:   rates,
		sampler: newGenericSampler(extraRate, maxTps, &serviceSignatureComputer{}, &clientSampleRateApplier{rates: rates}),
	}
}

// Sample counts an incoming trace and tells if it is a sample which has to be kept.
func (ss *ServiceSampler) Sample(trace model.Trace, root *model.Span, env string) bool {
	return ss.sampler.Sample(trace, root, env)
}

// Run the sampler.
func (ss *ServiceSampler) Run() {
	ss.sampler.Run()
}

// Stop the sampler.
func (ss *ServiceSampler) Stop() {
	ss.sampler.Stop()
}
