// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/spf13/cobra"
)

type uploadResult struct {
	Version  string   `json:"version"`
	Build    string   `json:"build"`
	ID       string   `json:"id"`
	BuildID  string   `json:"build_id,omitempty"`
	State    string   `json:"state"`
	Warnings []string `json:"warnings,omitempty"`
}

func newBuildUploadCommand(a *app) *cobra.Command {
	var (
		app      target
		timeout  time.Duration
		interval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "upload PATH",
		Short: "Upload an .ipa to App Store Connect",
		Long: "Uploads PATH, an exported .ipa, through App Store Connect's build upload API: no altool,\n" +
			"Transporter or Xcode, so it runs on any runner with the API key. The version, build\n" +
			"number and bundle ID come from the app's Info.plist; --bundle-id, when given, has to\n" +
			"agree with it.\n\n" +
			"It waits until Apple has accepted the upload (COMPLETE) or refused it (FAILED), and\n" +
			"prints Apple's warnings either way. Processing the build comes after that: follow it\n" +
			"with apple build wait.",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if strings.ToLower(filepath.Ext(path)) != ".ipa" {
				return usageError("%s is not an .ipa; export the archive first", path)
			}

			built, err := openBuiltApp(path)
			if err != nil {
				return err
			}
			bundleID := built.value("CFBundleIdentifier")
			version := built.value("CFBundleShortVersionString")
			build := built.value("CFBundleVersion")
			built.close()

			if bundleID == "" || version == "" || build == "" {
				return fmt.Errorf("%s's Info.plist needs CFBundleIdentifier, CFBundleShortVersionString and CFBundleVersion", path)
			}
			if app.bundleID != "" && app.bundleID != bundleID {
				return fmt.Errorf("%s is %s, not %s", path, bundleID, app.bundleID)
			}
			app.bundleID = bundleID

			appID, err := app.appID(a)
			if err != nil {
				return err
			}
			client, err := a.api()
			if err != nil {
				return err
			}

			label := fmt.Sprintf("%s (%s)", version, build)
			a.logf("Uploading %s %s.", bundleID, label)
			upload, err := client.UploadBuild(appID, app.platform, version, build, path)
			if err != nil {
				return err
			}
			a.logf("Uploaded; Apple is checking it.")

			deadline := a.now().Add(timeout)
			for {
				client, err := a.api()
				if err != nil {
					return err
				}
				if upload, err = client.GetBuildUpload(upload.ID); err != nil {
					return err
				}

				state := upload.CurrentState()
				switch state {
				case models.BuildUploadStateComplete, models.BuildUploadStateFailed:
					result := uploadResult{Version: version, Build: build, ID: upload.ID, BuildID: upload.BuildID(), State: state}
					if details := upload.Attributes.State; details != nil {
						for _, warning := range details.Warnings {
							result.Warnings = append(result.Warnings, warning.Code+": "+warning.Description)
							a.logf("  ! %s: %s", warning.Code, warning.Description)
						}
						if state == models.BuildUploadStateFailed {
							return fmt.Errorf("apple refused %s: %s", label, orNone(apple.DescribeStateDetails(details.Errors)))
						}
					}
					if state == models.BuildUploadStateFailed {
						return fmt.Errorf("apple refused %s without saying why", label)
					}
					return a.emit(result, state)
				}

				if !a.now().Before(deadline) {
					return fmt.Errorf("gave up waiting; the upload of %s is %s", label, orNone(state))
				}
				a.logf("%s %s: %s", a.now().UTC().Format("15:04:05"), label, orNone(state))
				a.sleep(interval)
			}
		},
	}

	app.addFlags(cmd)
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Minute, "give up waiting for Apple's checks after this long")
	cmd.Flags().DurationVar(&interval, "interval", 15*time.Second, "how often to ask")
	return cmd
}
