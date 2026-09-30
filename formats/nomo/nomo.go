// Package nomo renders Nomo worksheets (.nomo) as typeset HTML pages.
//
// It does not reimplement Nomo. It runs Nomo's own WebAssembly module — the
// artifact the Nomo editor runs in the browser — under wazero, a pure-Go
// runtime, so the page served here is the one the editor draws and the one
// `nomo html --mathml` writes, byte for byte, and no cgo or external binary is
// needed. The module imports nothing at all, so it cannot reach the host.
//
// The module and the math font are embedded from assets/, which fetch.sh
// fills from a Nomo release and pins by hash.
//
//go:generate ./fetch.sh --verify
package nomo

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var (
	//go:embed assets/nomo.wasm
	module []byte

	// Font is the STIX Two Math subset the Nomo editor ships: the face the
	// typeset mathematics is laid out with. A host serves it at the URL it
	// passes to Render.
	//
	//go:embed assets/stix-two-math-subset.woff2
	Font []byte

	// FontLicense is the SIL Open Font License the font is under. It has to
	// travel with the font, so a host serving one should serve the other.
	//
	//go:embed assets/OFL.txt
	FontLicense []byte

	// Notice holds the licences of the code compiled into the module.
	//
	//go:embed assets/NOTICE.txt
	Notice []byte
)

// ModuleHash identifies the engine. A cache or an ETag keyed on a worksheet's
// source must include it, since a new engine can render the same source
// differently.
var ModuleHash = func() string {
	sum := sha256.Sum256(module)
	return hex.EncodeToString(sum[:])
}()

// Timeout bounds one render. The engine bounds its own work with fixed limits
// (call depth, range sizes), so this is a backstop against a pathological
// worksheet holding a server goroutine, not a budget a real worksheet meets.
var Timeout = 10 * time.Second

// The runtime and the compiled module are shared. Compiling 1.2 MB of wasm is
// the expensive step, so it happens once, on first use rather than at program
// start: a server that never sees a .nomo file never pays for it.
var (
	once     sync.Once
	rt       wazero.Runtime
	compiled wazero.CompiledModule
	initErr  error
)

func setup() {
	ctx := context.Background()
	rt = wazero.NewRuntimeWithConfig(ctx,
		wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	compiled, initErr = rt.CompileModule(ctx, module)
}

// Render evaluates a worksheet and returns it as a standalone HTML document
// with its mathematics typeset (MathML).
//
// title is used when the worksheet does not open with its own level-1
// heading. fontURL is where the page fetches the math font from (see Font);
// empty leaves it to the fonts the reader has installed.
//
// A worksheet with errors is not an error here: the diagnostics are drawn in
// the page. An error means the engine itself failed or ran out of time.
func Render(source, title, fontURL string) ([]byte, error) {
	once.Do(setup)
	if initErr != nil {
		return nil, fmt.Errorf("nomo: compiling the engine: %w", initErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	// A fresh instance per render. It costs a copy of the module's data, and
	// buys isolation: an instance that traps is left with its allocator
	// mid-update and cannot be reused, and concurrent requests need separate
	// memories anyway. The empty name lets instances coexist.
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("nomo: instantiating the engine: %w", err)
	}
	defer mod.Close(context.Background())

	args := make([]uint64, 0, 7)
	for _, s := range []string{source, title, fontURL} {
		ptr, n, err := write(ctx, mod, s)
		if err != nil {
			return nil, err
		}
		args = append(args, ptr, n)
	}
	args = append(args, 1) // mathml: the typeset view

	res, err := mod.ExportedFunction("nomo_render_html").Call(ctx, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("nomo: render exceeded %v", Timeout)
		}
		return nil, fmt.Errorf("nomo: render failed: %w", err)
	}
	return read(mod, uint32(res[0]))
}

// write copies s into the instance's memory and returns its (pointer, length).
// The instance is discarded after the call, so nothing is freed.
func write(ctx context.Context, mod api.Module, s string) (uint64, uint64, error) {
	if len(s) == 0 {
		return 0, 0, nil
	}
	res, err := mod.ExportedFunction("nomo_alloc").Call(ctx, uint64(len(s)))
	if err != nil || res[0] == 0 {
		return 0, 0, errors.New("nomo: allocation in the engine failed")
	}
	if !mod.Memory().Write(uint32(res[0]), []byte(s)) {
		return 0, 0, errors.New("nomo: allocation outside the engine's memory")
	}
	return res[0], uint64(len(s)), nil
}

// read copies out a result buffer: a little-endian u32 length, then the bytes.
func read(mod api.Module, ptr uint32) ([]byte, error) {
	if ptr == 0 {
		return nil, errors.New("nomo: the engine rejected its input (not UTF-8?)")
	}
	header, ok := mod.Memory().Read(ptr, 4)
	if !ok {
		return nil, errors.New("nomo: result outside the engine's memory")
	}
	body, ok := mod.Memory().Read(ptr+4, binary.LittleEndian.Uint32(header))
	if !ok {
		return nil, errors.New("nomo: result outside the engine's memory")
	}
	// Read returns a view into the instance's memory, which is about to close.
	return append([]byte(nil), body...), nil
}
