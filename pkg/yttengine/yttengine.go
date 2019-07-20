package yttengine

import (
	"fmt"

	cmdcore "github.com/k14s/ytt/pkg/cmd/core"
	"github.com/k14s/ytt/pkg/cmd/template"
	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chartutil"
)

// Engine is an implementation of 'cmd/tiller/environment'.Engine that uses Go templates.
type YttEngine struct {
	// If strict is enabled, template rendering will fail if a template references
	// a value that was not passed in.
	Strict bool
	// In LintMode, some 'required' template values may be missing, so don't fail
	LintMode bool
}

// Render takes a chart, optional values, and value overrides, and attempts to render the Go templates.
//
// Render can be called repeatedly on the same engine.
//
// This will look in the chart's 'templates' data (e.g. the 'templates/' directory)
// and attempt to render the templates there using the values passed in.
//
// Values are scoped to their templates. A dependency template will not have
// access to the values set for its parent. If chart "foo" includes chart "bar",
// "bar" will not have access to the values for "foo".
//
// Values should be prepared with something like `chartutils.ReadValues`.
//
// Values are passed through the templates according to scope. If the top layer
// chart includes the chart foo, which includes the chart bar, the values map
// will be examined for a table called "foo". If "foo" is found in vals,
// that section of the values will be passed into the "foo" chart. And if that
// section contains a value named "bar", that value will be passed on to the
// bar chart during render time.
func (e YttEngine) Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	var paths []string
	for _, f := range chrt.Templates {
		fmt.Println(f.Name)
		// TODO: figure out how to deal with chart path
		paths = append(paths, fmt.Sprintf("chart/%s", f.Name))
	}
	println("----")

	fileSource := template.NewRegularFilesSourceExpanded(
		paths,
		[]string{},
		false,
		"",
		"yaml",
		cmdcore.NewPlainUI(false),
	)

	opt := template.NewOptions()
	templates, err := fileSource.Input()
	if err != nil {
		return nil, err
	}

	outputs := opt.RunWithFiles(templates, cmdcore.NewPlainUI(false))
	// fileSource.Output(outputs)

	result := make(map[string]string)
	for _, output := range outputs.Files {
		result[output.RelativePath()] = string(output.Bytes())
	}

	println("Succeeded")
	return result, nil
}

func Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	return new(YttEngine).Render(chrt, values)
}
