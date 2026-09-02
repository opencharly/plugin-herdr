// Command serve is the OUT-OF-PROCESS placement shim for the herdr plugin:
// `charly` fork/execs this binary with the pass-through tokens after
// `charly herdr` when the plugin is served out-of-process. It runs the SAME
// effect as the compiled-in Invoke(OpRun) path (CliMain), so both placements
// are placement-invisible.
package main

import (
	"github.com/opencharly/sdk"

	herdr "github.com/opencharly/plugin-herdr/candy/plugin-herdr"
)

func main() {
	sdk.Main(herdr.NewProvider(), herdr.NewMeta(), herdr.CliMain)
}
