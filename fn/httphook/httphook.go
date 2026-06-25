// Package httphook is a dependency-free registry for optional HTTP request
// interceptors that operate over an fn.FNode file root.
//
// Each integration registers an interceptor from its own init(), and the host
// server runs every registered interceptor before its normal file resolution.
// This keeps the host free of any import of the integrations themselves: a new
// interceptor is enabled by blank importing its adapter package, nothing else.
//
// It is the request-handling analogue of gserver's context registry (ctxreg),
// but its signature references golib's fn.FNode rather than the host server, so
// adapters can live in golib without importing the host.
package httphook

import (
	"net/http"

	"github.com/rveen/golib/fn"
)

// Interceptor optionally handles a request before normal file resolution.
// root is the server's file root (resolution proceeds as in normal serving).
// It returns true when it fully handled the request (response written or
// errored), and false to let normal handling proceed.
type Interceptor func(root *fn.FNode, w http.ResponseWriter, r *http.Request, reqPath string) bool

// interceptors holds the registered hooks in registration order.
var interceptors []Interceptor

// Register records an interceptor. It is meant to be called from the init() of
// an adapter package. Order of registration is order of execution.
func Register(f Interceptor) { interceptors = append(interceptors, f) }

// All returns the registered interceptors. The returned slice is the live
// registry; callers must not mutate it.
func All() []Interceptor { return interceptors }
