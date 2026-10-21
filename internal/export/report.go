// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// NotesFile is the file the notes are written to, beside the configuration.
const NotesFile = "EXPORT_NOTES.md"

// providersFile pins the provider. Import blocks need Terraform 1.5.
func providersFile() *file {
	f := newFile("providers.tf")
	f.comment("Written by `apple terraform export`. Credentials come from the\nAPPLE_APP_STORE_CONNECT_* environment variables, as they did for the export.")

	terraform := f.hcl.Body().AppendNewBlock("terraform", nil).Body()
	terraform.SetAttributeValue("required_version", cty.StringVal(">= 1.5"))
	providers := terraform.AppendNewBlock("required_providers", nil).Body()
	providers.SetAttributeValue("apple", cty.ObjectVal(map[string]cty.Value{
		"source":  cty.StringVal("ahmedosman00/apple"),
		"version": cty.StringVal("~> 0.1"),
	}))
	f.hcl.Body().AppendNewline()
	f.hcl.Body().AppendNewBlock("provider", []string{"apple"})

	return f
}

// variablesFile declares the secrets Apple does not hand back. Each defaults
// to null, which is what the import records.
func variablesFile(names []string) *file {
	f := newFile("variables.tf")
	for i, name := range names {
		if i > 0 {
			f.hcl.Body().AppendNewline()
		}
		v := f.hcl.Body().AppendNewBlock("variable", []string{name}).Body()
		v.SetAttributeTraversal("type", hcl.Traversal{hcl.TraverseRoot{Name: "string"}})
		v.SetAttributeValue("sensitive", cty.True)
		v.SetAttributeValue("default", cty.NullVal(cty.String))
		v.SetAttributeValue("description", cty.StringVal(
			"Apple does not return this password, so the export could not. Left null, Terraform leaves the one Apple holds alone."))
	}
	return f
}

// renderNotes is the human-readable account of the export: what the first
// plan should say, and everything that is not in the configuration.
func renderNotes(result *Result) []byte {
	var b strings.Builder

	b.WriteString("# Export notes\n\n")
	b.WriteString("This configuration was written by `apple terraform export` from what App Store Connect holds.\n")
	b.WriteString("Every resource has an `import` block beside it, so nothing is created:\n\n")
	b.WriteString("```sh\nterraform init\nterraform plan\n```\n\n")
	fmt.Fprintf(&b, "The plan should report **%d to import, 0 to add, 0 to change, 0 to destroy**. "+
		"Anything else is something the export got wrong. Do not apply a plan that destroys or replaces anything; "+
		"fix the configuration first. Once applied, the `import` blocks can be deleted.\n\n", result.ImportCount())

	b.WriteString("## Imported\n\n| Resource | Count |\n|---|---|\n")
	types := make([]string, 0, len(result.Imports))
	for t := range result.Imports {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Fprintf(&b, "| `%s` | %d |\n", t, result.Imports[t])
	}
	b.WriteString("\n")

	for _, section := range []struct {
		severity Severity
		heading  string
	}{
		{Attention, "Needs attention"},
		{Skipped, "Not in the configuration"},
	} {
		var lines []string
		for _, note := range result.Notes {
			if note.Severity != section.severity {
				continue
			}
			if note.Address != "" {
				lines = append(lines, fmt.Sprintf("- `%s`: %s", note.Address, note.Message))
			} else {
				lines = append(lines, "- "+note.Message)
			}
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", section.heading, strings.Join(lines, "\n"))
	}

	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}
