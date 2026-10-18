// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newDocsCommand writes one markdown page per command. It is hidden: `make
// generate` runs it, and CI fails when the pages and the commands disagree.
func newDocsCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-docs DIR",
		Hidden: true,
		Args:   exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.RemoveAll(args[0]); err != nil {
				return err
			}
			if err := os.MkdirAll(args[0], 0o755); err != nil {
				return err
			}
			root.DisableAutoGenTag = true
			return doc.GenMarkdownTree(root, args[0])
		},
	}
}
