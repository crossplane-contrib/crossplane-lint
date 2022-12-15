package main

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	errFmtInvalidFormatParameter = "invalid format parameter '%s'"
)

// Set by goreleaser.
// See https://goreleaser.com/cookbooks/using-main.version?h=ldf.
var (
	version string = "dev"
	commit  string = "none"
	date    string = "unknown"
)

type versionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
}

type versionCmd struct {
	Output string `short:"o" enum:"yaml,json" default:"yaml" help:"Defines the output format of the version information."`
}

func (c *versionCmd) Run() error {
	v := &versionInfo{
		Version:   version,
		GitCommit: commit,
		BuildDate: date,
	}

	output, err := c.formatOutput(v)
	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}

func (c *versionCmd) formatOutput(v *versionInfo) ([]byte, error) {
	switch c.Output {
	case "yaml":
		return yaml.Marshal(v)
	case "json":
		return json.Marshal(v)
	}
	return nil, errors.Errorf(errFmtInvalidFormatParameter, c.Output)
}
