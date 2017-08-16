package sampler

import (
	"time"
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

	sampler *Sampler
}

// NewServiceSampler returns a new service sampler.
func NewServiceSampler(extraRate, maxTps float64) *ServiceSampler {
	return &ServiceSampler{
		Rates:   NewRateByService(DefaultServiceSamplerTimeout),
		sampler: NewSampler(extraRate, maxTps, &ServiceSignatureComputer{}),
	}
}
