package sampler

import (
	"hash/fnv"

	"github.com/DataDog/datadog-trace-agent/model"
)

// combinedSignatureComputer allows signature computing using as many hints as
// possible to get a unique category for traces.
type combinedSignatureComputer struct {
}

// ComputeSignatureWithRootAndEnv generates the signature of a trace knowing its root
// Signature based on the hash of (env, service, name, resource, is_error) for the root, plus the set of
// (env, service, name, is_error) of each span.
func (csc *combinedSignatureComputer) ComputeSignatureWithRootAndEnv(trace model.Trace, root *model.Span, env string) Signature {
	rootHash := computeRootHash(*root, env)
	spanHashes := make([]spanHash, 0, len(trace))

	for i := range trace {
		spanHashes = append(spanHashes, computeSpanHash(trace[i], env))
	}

	// Now sort, dedupe then merge all the hashes to build the signature
	sortHashes(spanHashes)

	last := spanHashes[0]
	traceHash := last ^ rootHash
	for i := 1; i < len(spanHashes); i++ {
		if spanHashes[i] != last {
			last = spanHashes[i]
			traceHash = spanHashes[i] ^ traceHash
		}
	}

	return Signature(traceHash)
}

// ComputeSignature is the same as ComputeSignatureWithRoot, except that it finds the root itself
func (csc *combinedSignatureComputer) ComputeSignature(trace model.Trace) Signature {
	root := trace.GetRoot()
	env := trace.GetEnv()

	return csc.ComputeSignatureWithRootAndEnv(trace, root, env)
}

func computeSpanHash(span model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(env))
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}

func computeRootHash(span model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(env))
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte(span.Resource))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}
