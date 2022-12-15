package lint

import (
	"golang.org/x/sync/errgroup"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/linter/rules"
	lintschema "github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/schema"
)

type linterContext struct {
	ruleName    string
	issueChan   chan lint.Issue
	schemaStore *lintschema.SchemaStore
}

func (c *linterContext) ReportIssue(issue lint.Issue) {
	issue.RuleName = c.ruleName
	c.issueChan <- issue
}

func (c *linterContext) GetCRDSchema(gvk schema.GroupVersionKind) *extv1.CustomResourceDefinitionVersion {
	return c.schemaStore.GetCRDSchema(gvk)
}

var defaultRules = map[string]LinterRule{
	"generic.checkDuplicates":         LinterRuleFunc(rules.CheckDuplicateObjects),
	"composition.checkCompositeType":  LinterRuleFunc(rules.CheckCompositionCompositeTypeRef),
	"composition.checkPathFieldPaths": LinterRuleFunc(rules.CheckCompositionFieldPaths),
}

var _ lint.Linter = &linter{}

type LinterRule interface {
	Validate(ctx lint.LinterContext, pkg *xpkg.Package)
}

type LinterRuleFunc func(ctx lint.LinterContext, pkg *xpkg.Package)

func (f LinterRuleFunc) Validate(ctx lint.LinterContext, pkg *xpkg.Package) {
	f(ctx, pkg)
}

type linter struct {
	schemaValidator *lintschema.SchemaStore
	rules           map[string]LinterRule
}

func Newlinter(schemaValidator *lintschema.SchemaStore) lint.Linter {
	return &linter{
		schemaValidator: schemaValidator,
		rules:           defaultRules,
	}
}

func (l *linter) Lint(pkg *xpkg.Package) lint.LinterReport {
	issueChan := l.runRulesConcurrently(pkg)
	report := lint.LinterReport{}
	for iss := range issueChan {
		report.Issues = append(report.Issues, iss)
	}
	return report
}

func (l *linter) runRulesConcurrently(pkg *xpkg.Package) chan lint.Issue {
	issueChan := make(chan lint.Issue)
	eg := errgroup.Group{}

	for name, r := range l.rules {
		ctx := &linterContext{
			ruleName:    name,
			issueChan:   issueChan,
			schemaStore: l.schemaValidator,
		}
		rule := r
		eg.Go(func() error {
			rule.Validate(ctx, pkg)
			return nil
		})
	}
	go func() {
		_ = eg.Wait()
		close(issueChan)
	}()
	return issueChan
}
