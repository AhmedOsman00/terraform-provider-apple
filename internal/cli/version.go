// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"sort"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/spf13/cobra"
)

// target is the app a command acts on, named the way a pipeline knows it.
type target struct {
	bundleID string
	platform string
}

func (t *target) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.bundleID, "bundle-id", "", "the app's bundle ID")
	cmd.Flags().StringVar(&t.platform, "platform", "IOS", "IOS, MAC_OS, TV_OS or VISION_OS")
}

func (t *target) appID(a *app) (string, error) {
	if t.bundleID == "" {
		return "", usageError("--bundle-id names the app (or APPLE_BUNDLE_ID)")
	}
	client, err := a.api()
	if err != nil {
		return "", err
	}
	found, err := client.GetAppByBundleID(t.bundleID)
	if err != nil {
		return "", fmt.Errorf("no App Store Connect app for %s: %w", t.bundleID, err)
	}
	return found.ID, nil
}

func (t *target) versions(a *app, appID string) ([]models.AppStoreVersion, error) {
	client, err := a.api()
	if err != nil {
		return nil, err
	}
	return client.GetAppStoreVersions(appID, t.platform)
}

func state(version models.AppStoreVersion) string {
	if version.Attributes.AppVersionState == nil {
		return ""
	}
	return *version.Attributes.AppVersionState
}

// newestReleased is the highest released version string, or "" before the
// first release.
func newestReleased(versions []models.AppStoreVersion) string {
	newest := ""
	for _, version := range versions {
		if releasedStates[state(version)] &&
			(newest == "" || compareVersions(version.Attributes.VersionString, newest) > 0) {
			newest = version.Attributes.VersionString
		}
	}
	return newest
}

func newVersionCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Work out, read and set the marketing version and build number",
	}
	cmd.AddCommand(newVersionNextCommand(a), newVersionSetCommand(a), newVersionCurrentCommand(a))
	return cmd
}

type nextResult struct {
	Version        string `json:"version"`
	Build          string `json:"build"`
	NewestReleased string `json:"newest_released,omitempty"`
	HighestBuild   int    `json:"highest_build"`
}

func newVersionNextCommand(a *app) *cobra.Command {
	var (
		app             target
		project         string
		keepVersion     bool
		yearlyMajorFrom int
	)

	cmd := &cobra.Command{
		Use:   "next [VERSION]",
		Short: "Print the next version and build number App Store Connect will accept",
		Long: "Prints \"VERSION BUILD\" on stdout, and what it found on stderr. Nothing is changed.\n\n" +
			"Without VERSION it is the minor after the newest released version: 1.2.0 is followed by\n" +
			"1.3.0. With --yearly-major-from, the first release of a calendar year takes the next\n" +
			"major instead, the years since that one plus one. Before anything is released it is\n" +
			"the project's MARKETING_VERSION. A VERSION given is refused if it has been released, or\n" +
			"is not above the newest released version.\n\n" +
			"The build is the highest build number already uploaded for that version plus one, or 1.",
		Args: maxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requested := ""
			if len(args) == 1 {
				requested = args[0]
			}
			if keepVersion {
				if requested != "" {
					return usageError("--keep-version takes no VERSION")
				}
				current, _, err := projectVersions(project)
				if err != nil {
					return err
				}
				requested = current
			}
			if requested != "" {
				if err := checkVersion(requested); err != nil {
					return err
				}
			}

			appID, err := app.appID(a)
			if err != nil {
				return err
			}
			versions, err := app.versions(a, appID)
			if err != nil {
				return err
			}

			sort.Slice(versions, func(i, j int) bool {
				return compareVersions(versions[i].Attributes.VersionString, versions[j].Attributes.VersionString) > 0
			})
			a.logf("App Store versions:")
			for _, version := range versions {
				a.logf("  %s\t%s", version.Attributes.VersionString, state(version))
			}

			newest := newestReleased(versions)
			var version string
			switch {
			case requested != "":
				if newest != "" && compareVersions(requested, newest) <= 0 {
					return fmt.Errorf("%s is not above the newest released version, %s, and Apple only takes higher ones",
						requested, newest)
				}
				version = requested
			case newest != "":
				version = nextVersion(newest, a.now().UTC().Year(), yearlyMajorFrom)
			default:
				current, _, err := projectVersions(project)
				if err != nil {
					return fmt.Errorf("nothing is released yet, so the first version is the project's: %w", err)
				}
				version = current
			}

			client, err := a.api()
			if err != nil {
				return err
			}
			builds, err := client.GetBuilds(appID, app.platform, version, "")
			if err != nil {
				return err
			}
			numbers := make([]string, 0, len(builds))
			for _, build := range builds {
				if build.Attributes.Version != nil {
					numbers = append(numbers, *build.Attributes.Version)
				}
			}
			highest, err := highestBuild(numbers)
			if err != nil {
				return err
			}

			result := nextResult{
				Version:        version,
				Build:          fmt.Sprint(highest + 1),
				NewestReleased: newest,
				HighestBuild:   highest,
			}
			a.logf("Newest released: %s", orNone(newest))
			a.logf("Highest build of %s: %s", version, orNone(fmt.Sprint(highest)))
			return a.emit(result, result.Version+" "+result.Build)
		},
	}

	app.addFlags(cmd)
	cmd.Flags().StringVar(&project, "project", "", "the .xcodeproj, read for --keep-version and before the first release")
	cmd.Flags().BoolVar(&keepVersion, "keep-version", false, "keep the project's version and only work out the next build, for a TestFlight build")
	cmd.Flags().IntVar(&yearlyMajorFrom, "yearly-major-from", 0, "the launch year; the first release of each later year takes the next major")
	return cmd
}

func newVersionSetCommand(a *app) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "set VERSION BUILD",
		Short: "Write MARKETING_VERSION and CURRENT_PROJECT_VERSION on every target",
		Long: "Writes the pair onto every build configuration in the project. It asks Apple nothing;\n" +
			"take the pair from `apple version next`.",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, build := args[0], args[1]
			if err := checkVersion(version); err != nil {
				return err
			}
			if !buildPattern.MatchString(build) {
				return usageError("a build number is dot-separated numbers, such as 42 or 1.2.3; got %q", build)
			}

			count, err := setProjectVersions(project, version, build)
			if err != nil {
				return err
			}
			a.logf("Set %s (%s) on %d build configurations.", version, build, count)
			return a.emit(map[string]any{"version": version, "build": build, "configurations": count},
				version+" "+build)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "the .xcodeproj")
	return cmd
}

func newVersionCurrentCommand(a *app) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the project's MARKETING_VERSION and CURRENT_PROJECT_VERSION",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			version, build, err := projectVersions(project)
			if err != nil {
				return err
			}
			return a.emit(map[string]string{"version": version, "build": build}, version+" "+build)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "the .xcodeproj")
	return cmd
}

func orNone(value string) string {
	if value == "" || value == "0" {
		return "none"
	}
	return value
}
