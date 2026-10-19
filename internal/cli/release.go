// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/spf13/cobra"
)

// maxWhatsNew is Apple's limit on a version's release notes, in characters.
const maxWhatsNew = 4000

func newReleaseCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Check an App Store release before it starts, and submit it for review",
	}
	cmd.AddCommand(newReleasePreflightCommand(a), newReleaseSubmitCommand(a))
	return cmd
}

type preflightResult struct {
	Version         string   `json:"version"`
	First           bool     `json:"first"`
	NewestReleased  string   `json:"newest_released,omitempty"`
	EditableVersion string   `json:"editable_version,omitempty"`
	Locales         []string `json:"locales,omitempty"`
}

func newReleasePreflightCommand(a *app) *cobra.Command {
	var (
		app      target
		notesDir string
		locales  []string
	)

	cmd := &cobra.Command{
		Use:   "preflight VERSION",
		Short: "Check that App Store Connect will take VERSION, before anything is changed",
		Long: "Checks, and reports every problem rather than the first:\n\n" +
			"  - VERSION has not been released, and is above the newest released version\n" +
			"  - no other version is in review, which would block the next one until it is\n" +
			"    withdrawn or released\n" +
			"  - release notes exist for every locale, at NOTES_DIR/VERSION/LOCALE.txt, within\n" +
			"    Apple's 4000 characters. Apple rejects notes on a first version and requires them on\n" +
			"    every one after, so a first version needs none.\n\n" +
			"The locales are the ones App Store Connect has for the app's latest version, unless\n" +
			"--locales names them. A version being prepared under another number is reported, not\n" +
			"refused: the provider renames it in place.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]
			if err := checkVersion(version); err != nil {
				return err
			}

			appID, err := app.appID(a)
			if err != nil {
				return err
			}
			versions, err := app.versions(a, appID)
			if err != nil {
				return err
			}

			result := preflightResult{Version: version, NewestReleased: newestReleased(versions)}
			result.First = result.NewestReleased == ""

			var problems []string
			if !result.First && compareVersions(version, result.NewestReleased) <= 0 {
				problems = append(problems, fmt.Sprintf("%s is not above the newest released version, %s.",
					version, result.NewestReleased))
			}

			var latest *models.AppStoreVersion
			for i, candidate := range versions {
				s := state(candidate)
				switch {
				case editableStates[s]:
					result.EditableVersion = candidate.Attributes.VersionString
				case !releasedStates[s]:
					problems = append(problems, fmt.Sprintf("%s is %s. Withdraw it or let it be released first.",
						candidate.Attributes.VersionString, s))
				}
				if latest == nil || compareVersions(candidate.Attributes.VersionString, latest.Attributes.VersionString) > 0 {
					latest = &versions[i]
				}
			}

			if !result.First {
				result.Locales = locales
				if len(result.Locales) == 0 && latest != nil {
					result.Locales, err = versionLocales(a, latest.ID)
					if err != nil {
						return err
					}
				}
				problems = append(problems, checkNotes(notesDir, version, result.Locales)...)
			}

			a.logf("Release %s", version)
			a.logf("  Newest released: %s", orNone(result.NewestReleased))
			if result.EditableVersion != "" && result.EditableVersion != version {
				a.logf("  Being prepared: %s, which the provider renames to %s", result.EditableVersion, version)
			}
			if result.First {
				a.logf("  Release notes: not needed, it is the first version")
			} else {
				a.logf("  Release notes: %s", strings.Join(result.Locales, ", "))
			}

			if len(problems) > 0 {
				for _, problem := range problems {
					a.logf("  ✗ %s", problem)
				}
				return errors.New("preflight failed")
			}
			return a.emit(result, "ok")
		},
	}

	app.addFlags(cmd)
	cmd.Flags().StringVar(&notesDir, "notes-dir", "", "the directory holding VERSION/LOCALE.txt release notes")
	cmd.Flags().StringSliceVar(&locales, "locales", nil, "the locales that need notes, instead of the ones App Store Connect lists")
	return cmd
}

func versionLocales(a *app, versionID string) ([]string, error) {
	client, err := a.api()
	if err != nil {
		return nil, err
	}
	localizations, err := client.GetAppStoreVersionLocalizations(versionID)
	if err != nil {
		return nil, err
	}

	locales := make([]string, 0, len(localizations))
	for _, localization := range localizations {
		locales = append(locales, localization.Attributes.Locale)
	}
	sort.Strings(locales)
	return locales, nil
}

func checkNotes(dir, version string, locales []string) []string {
	if dir == "" {
		return []string{"--notes-dir is needed: every version after the first needs release notes."}
	}

	var problems []string
	for _, locale := range locales {
		path := filepath.Join(dir, version, locale+".txt")
		content, err := os.ReadFile(path)
		switch {
		case err != nil || strings.TrimSpace(string(content)) == "":
			problems = append(problems, fmt.Sprintf("Missing %s. Apple requires release notes in every locale after the first version.", path))
		case utf8.RuneCount([]byte(strings.TrimSpace(string(content)))) > maxWhatsNew:
			problems = append(problems, fmt.Sprintf("%s is over Apple's %d characters.", path, maxWhatsNew))
		}
	}
	return problems
}
