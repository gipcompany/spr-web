package sprbridge

import "runtime/debug"

// SprVersion returns the version of the embedded spr module.
func SprVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/ejoffe/spr" {
			return dep.Version
		}
	}
	return "unknown"
}
