// Copied from https://github.com/caddyserver/xcaddy/blob/b7fd102f41e12be4735dc77b0391823989812ce8/environment.go#L251
package main

import (
	"log/slog"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/mercure/caddy"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap/exp/zapslog"
)

func main() {
	// Backport of https://github.com/caddyserver/caddy/pull/6809
	// Remove this block when Caddy 2.10 will be released.
	_, _ = memlimit.SetGoMemLimitWithOpts(
		memlimit.WithLogger(
			slog.New(zapslog.NewHandler(caddy.Log().Core())),
		),
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
	)

	caddycmd.Main()
}
