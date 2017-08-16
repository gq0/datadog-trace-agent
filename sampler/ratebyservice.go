package sampler

import (
	"sync"
	"time"
)

const (
	defaultServiceRate    = 1
	defaultServiceRateKey = "service:,env:"
)

type timeoutValue struct {
	deadline time.Time
	value    float64
}

func byServiceKey(service, env string) string {
	return "service:" + service + ",env:" + env
}

// RateByService stores the sampling rate per service.
type RateByService struct {
	timeout time.Duration
	rates   map[string]timeoutValue
	mutex   sync.RWMutex
}

// NewRateByService creates a new rate by service object.
func NewRateByService(timeout time.Duration) *RateByService {
	return &RateByService{
		timeout: timeout,
	}
}

// Set the sampling rate for a service.
func (rbs *RateByService) Set(service, env string, rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}

	rbs.mutex.Lock()
	defer rbs.mutex.Unlock()

	if rbs.rates == nil {
		rbs.rates = make(map[string]timeoutValue, 1)
	}
	timeout := rbs.timeout
	if timeout <= 0 {
		timeout = 365 * 24 * time.Hour // if no timeout given, (almost) never expire
	}
	rbs.rates[byServiceKey(service, env)] = timeoutValue{
		deadline: time.Now().Add(timeout),
		value:    rate,
	}
}

// Get the sampling rate for a service.
func (rbs *RateByService) Get(service, env string) float64 {
	rbs.mutex.RLock()
	defer rbs.mutex.RUnlock()

	if rbs.rates == nil {
		return defaultServiceRate
	}
	if tv, ok := rbs.rates[byServiceKey(service, env)]; ok {
		return tv.value
	}
	return defaultServiceRate
}

// GetAll returns all sampling rates for all services.
func (rbs *RateByService) GetAll() map[string]float64 {
	rbs.mutex.RLock()
	defer rbs.mutex.RUnlock()

	ret := make(map[string]float64, len(rbs.rates)+1)
	ret[defaultServiceRateKey] = defaultServiceRate
	now := time.Now()
	for k, v := range rbs.rates {
		if now.Before(v.deadline) {
			ret[k] = v.value
		}
	}

	return ret
}
