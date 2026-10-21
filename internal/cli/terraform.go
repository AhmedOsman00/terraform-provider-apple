// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
			"--only takes signing, app, products, testflight and team. app, products and testflight are\n" +
			"per app: every app on the team, or those named by --app. Nothing is written to Apple, and\n" +
			"terraform is not run.",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			var domains []export.Domain
			for _, name := range only {
				domain, err := export.ParseDomain(name)
				if err != nil {
					return usageError("--only: %v", err)
				}
				domains = append(domains, domain)
			}

			if err := checkOutputDirectory(out, force); err != nil {
				return err
			}

			client, err := a.api()
			if err != nil {
				return err
			}

			result, err := export.Run(client, export.Options{
				Apps:     apps,
				Domains:  domains,
				Progress: a.logf,
			})
			if err != nil {
				return err
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

	cmd.Flags().StringVar(&out, "out", "", "the directory to write into; it must be empty or not exist yet")
	cmd.Flags().StringSliceVar(&apps, "app", nil, "an app to export, by App Store Connect ID or bundle identifier; repeat for several (default every app)")
	cmd.Flags().StringSliceVar(&only, "only", nil, "export only these parts: signing, app, products, testflight, team (default all)")
	cmd.Flags().BoolVar(&force, "force", false, "write into --out even when it already holds files, replacing any of the same name")
	_ = cmd.MarkFlagRequired("out")
	return cmd
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
