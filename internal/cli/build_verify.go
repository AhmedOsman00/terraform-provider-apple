// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type verifyResult struct {
	Path         string         `json:"path"`
	BundleID     string         `json:"bundle_id"`
	Version      string         `json:"version"`
	Build        string         `json:"build"`
	Entitlements map[string]any `json:"entitlements,omitempty"`
	Problems     []string       `json:"problems,omitempty"`
}

func newBuildVerifyCommand(a *app) *cobra.Command {
	var (
		bundleID     string
		version      string
		build        string
		entitlements []string
	)

	cmd := &cobra.Command{
		Use:   "verify PATH",
		Short: "Check that an archive or .ipa is the build you meant to make",
		Long: "Reads the app inside PATH, an .xcarchive, an .ipa or an .app, and checks it against the\n" +
			"flags given, reporting every mismatch rather than the first:\n\n" +
			"  --bundle-id    CFBundleIdentifier\n" +
			"  --version      CFBundleShortVersionString\n" +
			"  --build        CFBundleVersion\n" +
			"  --entitlement  KEY=VALUE, an entitlement the app was signed with. A boolean matches\n" +
			"                 true or false, and a list matches when VALUE is one of its entries.\n" +
			"                 Repeat it for more.\n\n" +
			"Nothing is unzipped but the executable, and only for --entitlement. Entitlements are read\n" +
			"with codesign, so that check needs macOS; the others run anywhere.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wanted := make([][2]string, 0, len(entitlements))
			for _, entitlement := range entitlements {
				key, value, ok := strings.Cut(entitlement, "=")
				if !ok || key == "" {
					return usageError("--entitlement takes KEY=VALUE, not %q", entitlement)
				}
				wanted = append(wanted, [2]string{key, value})
			}

			built, err := openBuiltApp(args[0])
			if err != nil {
				return err
			}
			defer built.close()

			result := verifyResult{
				Path:     args[0],
				BundleID: built.value("CFBundleIdentifier"),
				Version:  built.value("CFBundleShortVersionString"),
				Build:    built.value("CFBundleVersion"),
			}
			for _, check := range [][3]string{
				{"CFBundleIdentifier", result.BundleID, bundleID},
				{"CFBundleShortVersionString", result.Version, version},
				{"CFBundleVersion", result.Build, build},
			} {
				if check[2] != "" && check[1] != check[2] {
					result.Problems = append(result.Problems, fmt.Sprintf("%s is %s, expected %s.", check[0], orNone(check[1]), check[2]))
				}
			}

			if len(wanted) > 0 {
				signed, err := a.readEntitlements(built)
				if err != nil {
					return err
				}
				result.Entitlements = signed
				for _, entitlement := range wanted {
					if problem := entitlementProblem(signed, entitlement[0], entitlement[1]); problem != "" {
						result.Problems = append(result.Problems, problem)
					}
				}
			}

			a.logf("Build %s %s (%s)", orNone(result.BundleID), orNone(result.Version), orNone(result.Build))
			if len(result.Problems) == 0 {
				return a.emit(result, "ok")
			}
			for _, problem := range result.Problems {
				a.logf("  ✗ %s", problem)
			}
			if a.json {
				if err := a.emit(result, ""); err != nil {
					return err
				}
			}
			return errors.New("verify failed")
		},
	}

	cmd.Flags().StringVar(&bundleID, "bundle-id", "", "the CFBundleIdentifier to expect")
	cmd.Flags().StringVar(&version, "version", "", "the CFBundleShortVersionString to expect")
	cmd.Flags().StringVar(&build, "build", "", "the CFBundleVersion to expect")
	cmd.Flags().StringArrayVar(&entitlements, "entitlement", nil, "an entitlement to expect, as KEY=VALUE; repeatable")
	return cmd
}

// readEntitlements runs the entitlement reader on the app's executable.
func (a *app) readEntitlements(built *builtApp) (map[string]any, error) {
	executable, cleanup, err := built.executable()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	read := a.entitlements
	if read == nil {
		read = codesignEntitlements
	}
	return read(executable)
}

// entitlementProblem compares a signed entitlement with the text of a flag:
// a string must equal it, a boolean must print as it, and a list must hold it.
func entitlementProblem(signed map[string]any, key, want string) string {
	got, ok := signed[key]
	if !ok {
		return fmt.Sprintf("The app was not signed with %s; expected %s.", key, want)
	}

	switch value := got.(type) {
	case string:
		if value == want {
			return ""
		}
	case bool:
		if strconv.FormatBool(value) == want {
			return ""
		}
	case []any:
		entries := make([]string, 0, len(value))
		for _, entry := range value {
			if fmt.Sprint(entry) == want {
				return ""
			}
			entries = append(entries, fmt.Sprint(entry))
		}
		sort.Strings(entries)
		return fmt.Sprintf("%s is [%s], which does not include %s.", key, strings.Join(entries, ", "), want)
	default:
		if fmt.Sprint(value) == want {
			return ""
		}
	}
	return fmt.Sprintf("%s is %v, expected %s.", key, got, want)
}
