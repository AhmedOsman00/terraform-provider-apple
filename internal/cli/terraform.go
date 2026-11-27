// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/export"

	"github.com/spf13/cobra"
)

func newTerraformCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terraform",
		Short: "Write Terraform configuration for what the account already holds",
	}
	cmd.AddCommand(newTerraformExportCommand(a))
	return cmd
}

func newTerraformExportCommand(a *app) *cobra.Command {
	var (
		out   string
		apps  []string
		only  []string
		list  bool
		force bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write .tf files and import blocks from the live account, so the first plan changes nothing",
		Long: "Reads the App Store Connect account and writes Terraform configuration for it into --out:\n" +
			"a resource block per record holding the value Apple holds, an import block beside each,\n" +
			"and EXPORT_NOTES.md listing what was left out and why. `terraform plan` in that directory\n" +
			"should then import everything and change nothing, which is the point: values copied by\n" +
			"hand are how a first apply overwrites a price or replaces a product.\n\n" +
			"Certificates are read through data sources rather than imported, since importing one\n" +
			"reissues it and revokes the original. Screenshots are left out, since Apple keeps their\n" +
			"bytes and not the files. Demo account passwords become sensitive variables.\n\n" +
			"Without --app and --only, the whole account is exported. --app (by App Store Connect ID or\n" +
			"bundle identifier, repeatable) exports those apps' own parts: the listing, products and\n" +
			"TestFlight, the Bundle IDs under the app's own (com.acme.app and com.acme.app.*, except one\n" +
			"that is another app's), and the profiles issued for them. The certificates and devices those\n" +
			"profiles use are read through data sources, and the account's merchant IDs, pass type IDs,\n" +
			"users and invitations are left out unless --only names them. An explicit --only always wins:\n" +
			"--app com.acme.app --only users exports only users.\n\n" +
			"--list reads the account the same way and prints what would be written, by app and part,\n" +
			"without writing anything; --out is then not needed.\n\n" +
			"Parts (--only):\n" + export.DescribeParts() + "\n" +
			"Nothing is written to Apple, and terraform is not run.",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			var parts []export.Part
			for _, name := range only {
				named, err := export.ParsePart(name)
				if err != nil {
					return usageError("--only: %v", err)
				}
				for _, part := range named {
					if !slices.Contains(parts, part) {
						parts = append(parts, part)
					}
				}
			}

			if !list {
				if err := checkOutputDirectory(out, force); err != nil {
					return err
				}
			}

			client, err := a.api()
			if err != nil {
				return err
			}

			result, err := export.Run(client, export.Options{
				Apps:     apps,
				Parts:    parts,
				Progress: a.logf,
			})
			if err != nil {
				return err
			}

			if list {
				return a.emit(struct {
					Contents []export.Entry `json:"contents"`
					Notes    []export.Note  `json:"notes"`
				}{result.Contents, result.Notes}, renderContents(result))
			}

			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			for _, name := range result.FileNames() {
				if err := os.WriteFile(filepath.Join(out, name), result.Files[name], 0o644); err != nil {
					return err
				}
			}

			plain := fmt.Sprintf("Wrote %d files to %s: %d records to import, %d notes in %s.\n"+
				"Next: cd %s && terraform init && terraform plan — it should import everything and change nothing.",
				len(result.Files), out, result.ImportCount(), len(result.Notes), export.NotesFile, out)
			return a.emit(struct {
				Directory string         `json:"directory"`
				Files     []string       `json:"files"`
				Imports   map[string]int `json:"imports"`
				Notes     []export.Note  `json:"notes"`
			}{out, result.FileNames(), result.Imports, result.Notes}, plain)
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "the directory to write into; it must be empty or not exist yet (not needed with --list)")
	cmd.Flags().StringSliceVar(&apps, "app", nil, "an app to export, by App Store Connect ID or bundle identifier; repeat for several (default every app, and the whole account)")
	cmd.Flags().StringSliceVar(&only, "only", nil, "export only these parts, or the aliases signing, team and app; see the list above (default all, or under --app every part that is not the account's)")
	cmd.Flags().BoolVar(&list, "list", false, "print what would be written, by app and part, and write nothing")
	cmd.Flags().BoolVar(&force, "force", false, "write into --out even when it already holds files, replacing any of the same name")
	return cmd
}

// renderContents is --list: each app's blocks by part, then those of the
// account or several apps, then the notes, which carry the unclaimed Bundle
// IDs and everything else left out.
func renderContents(result *export.Result) string {
	type group struct {
		app   string
		parts map[export.Part][]export.Entry
		order []export.Part
	}
	var groups []*group
	byApp := map[string]*group{}
	for _, entry := range result.Contents {
		g, ok := byApp[entry.App]
		if !ok {
			g = &group{app: entry.App, parts: map[export.Part][]export.Entry{}}
			byApp[entry.App] = g
			groups = append(groups, g)
		}
		if _, ok := g.parts[entry.Part]; !ok {
			g.order = append(g.order, entry.Part)
		}
		g.parts[entry.Part] = append(g.parts[entry.Part], entry)
	}
	// The account's own blocks come last, after every app.
	slices.SortStableFunc(groups, func(x, y *group) int {
		switch {
		case x.app == "" && y.app != "":
			return 1
		case x.app != "" && y.app == "":
			return -1
		}
		return 0
	})

	var b strings.Builder
	for _, g := range groups {
		heading := g.app
		if heading == "" {
			heading = "Account, or shared by several apps"
		}
		fmt.Fprintf(&b, "%s\n", heading)
		for _, part := range g.order {
			fmt.Fprintf(&b, "  %s\n", part)
			for _, entry := range g.parts[part] {
				kind := "import"
				if !entry.Import {
					kind = "read  "
				}
				fmt.Fprintf(&b, "    %s %s  (%s)\n", kind, entry.Address, entry.File)
			}
		}
	}
	if len(result.Contents) == 0 {
		b.WriteString("Nothing to export.\n")
	}
	fmt.Fprintf(&b, "\n%d records to import.", result.ImportCount())
	if len(result.Notes) > 0 {
		fmt.Fprintf(&b, " %d notes:\n", len(result.Notes))
		for _, note := range result.Notes {
			if note.Address != "" {
				fmt.Fprintf(&b, "  - [%s] %s: %s\n", note.Severity, note.Address, note.Message)
			} else {
				fmt.Fprintf(&b, "  - [%s] %s\n", note.Severity, note.Message)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// checkOutputDirectory refuses to write over an existing configuration: an
// export landing in a directory that already manages the account would plan
// every record twice.
func checkOutputDirectory(dir string, force bool) error {
	if strings.TrimSpace(dir) == "" {
		return usageError("--out names the directory to write into")
	}

	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	case len(entries) > 0 && !force:
		return usageError("%s is not empty; export into a new directory, or pass --force to write into it anyway", dir)
	}
	return nil
}
