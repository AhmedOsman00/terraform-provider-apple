// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newTestFlightCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testflight",
		Short: "Write what TestFlight testers are told",
	}
	cmd.AddCommand(newTestFlightNotesCommand(a))
	return cmd
}

func newTestFlightNotesCommand(a *app) *cobra.Command {
	var (
		header string
		notes  string
		since  string
		limit  int
		dir    string
	)

	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Print a \"What to Test\" note from git: a header, notes, then the commit subjects",
		Long: "Prints a TestFlight \"What to Test\" note: --header, then --notes, then the subjects of\n" +
			"the commits since --since (or the last --limit), cut to Apple's 4000 characters. It reads\n" +
			"only git, so it runs anywhere. Hand the file to `apple build wait --what-to-test`.",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 1 {
				return usageError("--limit takes a positive number")
			}

			rangeArg := "HEAD"
			if since != "" {
				if err := exec.Command("git", "-C", dir, "cat-file", "-e", since+"^{commit}").Run(); err == nil {
					rangeArg = since + "..HEAD"
				} else {
					a.logf("No commit %s here; listing the last %d instead.", since, limit)
				}
			}

			out, err := exec.Command("git", "-C", dir, "log", "--no-merges", "--format=- %s",
				"-n", fmt.Sprint(limit), rangeArg).Output()
			if err != nil {
				return fmt.Errorf("git log: %w", err)
			}

			var parts []string
			for _, part := range []string{header, notes, strings.TrimSpace(string(out))} {
				if part = strings.TrimSpace(part); part != "" {
					parts = append(parts, part)
				}
			}
			note := truncateRunes(strings.Join(parts, "\n\n"), maxWhatToTest)
			return a.emit(map[string]string{"what_to_test": note}, note)
		},
	}

	cmd.Flags().StringVar(&header, "header", "", "the first paragraph, such as which build and lane this is")
	cmd.Flags().StringVar(&notes, "notes", "", "a paragraph under the header")
	cmd.Flags().StringVar(&since, "since", "", "list the commits after this one, up to --limit")
	cmd.Flags().IntVar(&limit, "limit", 20, "at most this many commits")
	cmd.Flags().StringVar(&dir, "repo", ".", "the git repository to read")
	return cmd
}
