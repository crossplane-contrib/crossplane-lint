package print

import (
	"fmt"
	"io"

	"github.com/gookit/color"
	"github.com/pkg/errors"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/jsonpath"
)

const (
	errEvaluateJSONPath = "failed to evaluate JSON Path"
	errGetYamlNode      = "failed to parse yaml"
	errParsePath        = "failed to parse JSON path"
	errFindPath         = "failed to evaluate JSON path"
)

var _ Printer = &TextPrinter{}

type TextPrinter struct {
	out io.Writer
}

func NewTextPrinter(out io.Writer) *TextPrinter {
	return &TextPrinter{
		out: out,
	}
}

func (p *TextPrinter) PrintReport(report lint.LinterReport) error {
	for _, issue := range report.Issues {
		if err := p.printIssue(issue); err != nil {
			return err
		}
		fmt.Fprintln(p.out, "")
	}
	return nil
}

func (p *TextPrinter) printIssue(issue lint.Issue) error {
	fmt.Fprintf(p.out, "[%s] %s\n", color.Red.Render(issue.RuleName), issue.Description)
	if issue.Entry == nil {
		return nil
	}
	if issue.Path == nil {
		fmt.Fprintln(p.out, color.Blue.Render(fmt.Sprintf("  in %s", issue.Entry.Source)))
		return nil
	}
	line, column, err := evalJSONPath(issue.Entry, issue.Path)
	if err != nil {
		return errors.Wrap(err, errEvaluateJSONPath)
	}
	fmt.Fprintf(p.out, "  %s: %s\n", issue.Path.String(), issue.PathValue)
	fmt.Fprintln(p.out, color.Blue.Render(fmt.Sprintf("  in %s:%d:%d", issue.Entry.Source, line, column)))
	return nil
}

func evalJSONPath(e *xpkg.PackageEntry, path jsonpath.JSONPath) (line, column int, err error) {
	yamlPath, err := yamlpath.NewPath(path.String())
	if err != nil {
		return 0, 0, errors.Wrap(err, errParsePath)
	}
	node, err := e.GetYamlNode()
	if err != nil {
		return 0, 0, errors.Wrap(err, errGetYamlNode)
	}
	pathNodes, err := yamlPath.Find(node)
	if err != nil {
		return 0, 0, errors.Wrap(err, errFindPath)
	}
	if len(pathNodes) == 0 {
		return 0, 0, nil
	}
	pathNode := pathNodes[0]
	return pathNode.Line, pathNode.Column, nil
}
