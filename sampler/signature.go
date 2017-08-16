package sampler

import (
	"sort"

	"github.com/DataDog/datadog-trace-agent/model"
)

// Signature is a simple representation of trace, used to identify simlar traces
type Signature uint64

// SignatureComputer is an abstraction to allow different algorithm to be used
// by samplers, it defines signature computing methods.
type SignatureComputer interface {
	// ComputeSignatureWithRootAndEnv generates the signature of a trace knowing its root
	ComputeSignatureWithRootAndEnv(trace model.Trace, root *model.Span, env string) Signature
	// ComputeSignature is the same as ComputeSignatureWithRoot, except that it finds the root itself
	ComputeSignature(trace model.Trace) Signature
}

// spanHash is the type of the hashes used during the computation of a signature
// Use FNV for hashing since it is super-cheap and we have no cryptographic needs
type spanHash uint32
type spanHashSlice []spanHash

func (p spanHashSlice) Len() int           { return len(p) }
func (p spanHashSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p spanHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func sortHashes(hashes []spanHash)         { sort.Sort(spanHashSlice(hashes)) }
