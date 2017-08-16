package sampler

import (
	"hash/fnv"

	"github.com/DataDog/datadog-trace-agent/model"
)

// ServiceSignatureComputer allows signature computing using only service and env.
// Used in distributed tracing to get feedback to client libraries.
type ServiceSignatureComputer struct {
}

// ComputeSignatureWithRootAndEnv generates the signature of a trace knowing its root
// Signature based on the (root) service only.
func (ssc *ServiceSignatureComputer) ComputeSignatureWithRootAndEnv(trace model.Trace, root *model.Span, env string) Signature {
	serviceHash := computeServiceHash(*root, env)

	return Signature(serviceHash)
}

// ComputeSignature is the same as ComputeSignatureWithRoot, except that it finds the root itself
func (ssc *ServiceSignatureComputer) ComputeSignature(trace model.Trace) Signature {
	root := trace.GetRoot()
	env := trace.GetEnv()

	return ssc.ComputeSignatureWithRootAndEnv(trace, root, env)
}

func computeServiceHash(span model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(span.Service))
	h.Write([]byte{','})
	h.Write([]byte(env))

	return spanHash(h.Sum32())
}
