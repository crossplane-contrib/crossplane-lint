package main

import (
	"os"
	"path/filepath"

	"github.com/go-log/log"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/crossplane-contrib/crossplane-lint/internal/config"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/fetch"
	linter "github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/linter"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/print"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/schema"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/parse"
)

const (
	errParsePackage            = "failed to parse package"
	errLoadPackageDependencies = "failed to load package dependencies"
	errLintPackage             = "failed to lint package"
	errLinterIssues            = "%d issues discovered during linting"
	errLoadConfig              = "failed to load config"
	errRegisterPackageSchema   = "failed to register package schemas"
)

type lintPackageCmd struct {
	Package string `short:"f" help:"Path to the package that should be linted" type:"existingDir" required:"true"`

	Config string `env:"CROSSPLANE-LINT_CONFIG" type:"path" help:"Path to the config file." default:".crossplane-lint.yaml"`
	Home   string `env:"CROSSPLANE-LINT_HOME" type:"path" help:"Path to the CROSSPLANE-LINT home directy."`
}

func (c *lintPackageCmd) Run(fs afero.Fs, logger log.Logger) error {
	parser := parse.NewPackageDirectoryParser(fs)

	pkg, err := parser.ParsePackage(c.Package)
	if err != nil {
		return errors.Wrap(err, errParsePackage)
	}

	config, err := c.getConfig(fs)
	if err != nil {
		return errors.Wrap(err, errLoadConfig)
	}

	imageCacheDir, err := c.getImageCacheDir()
	if err != nil {
		return err
	}
	fetcher := fetch.NewFsCacheFetcher(
		afero.NewBasePathFs(fs, imageCacheDir),
		fetch.NewRemoteFetcher(),
	)

	pkgDeps, err := parse.LoadPackageDependencies(config.AdditionalPackages, parse.NewPackageImageParser(fetcher))
	if err != nil {
		return errors.Wrap(err, errLoadPackageDependencies)
	}

	schemaStore := schema.NewSchemaStore()
	if err := schemaStore.RegisterPackage(pkg); err != nil {
		return errors.Wrap(err, errRegisterPackageSchema)
	}
	for _, dep := range pkgDeps {
		if err := schemaStore.RegisterPackage(dep); err != nil {
			return errors.Wrap(err, errRegisterPackageSchema)
		}
	}

	pkgLinter := linter.Newlinter(schemaStore)
	report := pkgLinter.Lint(pkg)
	if len(report.Issues) == 0 {
		return nil
	}
	printer := c.buildPrinter()
	if err := printer.PrintReport(report); err != nil {
		return err
	}
	return errors.Errorf(errLinterIssues, len(report.Issues))
}

func (c *lintPackageCmd) buildPrinter() print.Printer {
	return print.NewTextPrinter(os.Stdout)
}

func (c *lintPackageCmd) getImageCacheDir() (string, error) {
	var homeDir string

	if c.Home != "" {
		homeDir = c.Home
	} else {
		userHome, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		homeDir = filepath.Join(userHome, "crossplane-lint")
	}
	return filepath.Join(homeDir, "images"), nil
}

func (c *lintPackageCmd) getConfig(fs afero.Fs) (config.Configuration, error) {
	data, err := afero.ReadFile(fs, c.Config)
	if err != nil {
		return config.Configuration{}, err
	}
	con := config.Configuration{}
	if err := yaml.Unmarshal(data, &con); err != nil {
		return config.DefaultConfig, errorIgnore(err, os.IsNotExist)
	}
	return con, nil
}

func errorIgnore(err error, filter func(error) bool) error {
	if filter(err) {
		return nil
	}
	return err
}
