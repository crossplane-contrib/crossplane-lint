package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/go-log/log"
	fmtLog "github.com/go-log/log/fmt"
	"github.com/spf13/afero"
)

var cli struct {
	// Lint struct {
	// 	Package lintPackageCmd `cmd:"package" help:"Scan a package for issues"`
	// 	} `cmd:"lint"`
	Package lintPackageCmd `cmd:"package" help:"Scan a directory of compositions and XRDs"`
	Version versionCmd     `cmd:"version" help:"Print version information"`
}

var _ = kong.Must(&cli)

func main() {
	fs := afero.NewOsFs()
	logger := fmtLog.NewFromWriter(os.Stderr)

	ctx := kong.Parse(&cli,
		kong.Name("crossplane-lint"),
		kong.Description("Linting of crossplane compositions and XRDs"),
		kong.BindTo(fs, (*afero.Fs)(nil)),
		kong.BindTo(logger, (*log.Logger)(nil)),
	)

	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
