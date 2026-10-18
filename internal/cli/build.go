// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/spf13/cobra"
)

// maxWhatToTest is Apple's limit on a build's TestFlight "What to Test".
const maxWhatToTest = 4000

func newBuildCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Follow an uploaded build through App Store Connect",
	}
	cmd.AddCommand(newBuildWaitCommand(a))
	return cmd
}

type waitResult struct {
	Version string `json:"version"`
	Build   string `json:"build"`
	ID      string `json:"id"`
	State   string `json:"processing_state"`
}

func newBuildWaitCommand(a *app) *cobra.Command {
	var (
		app        target
		until      string
		whatToTest string
		locale     string
		timeout    time.Duration
		interval   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "wait VERSION BUILD",
		Short: "Wait for an uploaded build to exist, or to finish processing",
		Long: "Polls App Store Connect until the build exists (--until exists) or Apple has finished\n" +
			"processing it (--until valid, the default). A build has to be VALID before a version\n" +
			"can take it. Only the API is involved, so it runs on any runner.\n\n" +
			"--what-to-test sets the build's TestFlight \"What to Test\" from a file. If Apple refuses\n" +
			"while the build is still processing, it waits for VALID and tries again.",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, build := args[0], args[1]
			if until != "exists" && until != "valid" {
				return usageError("--until is exists or valid, not %q", until)
			}
			var notes string
			if whatToTest != "" {
				content, err := os.ReadFile(whatToTest)
				if err != nil {
					return usageError("no file %s", whatToTest)
				}
				notes = truncateRunes(string(content), maxWhatToTest)
			}

			appID, err := app.appID(a)
			if err != nil {
				return err
			}

			w := &waiter{a: a, appID: appID, platform: app.platform, version: version, build: build,
				deadline: a.now().Add(timeout), interval: interval}
			found, err := w.poll(until == "valid")
			if err != nil {
				return err
			}

			if notes != "" {
				if err := setWhatToTest(a, found.ID, locale, notes); err != nil {
					if processingState(*found) == "VALID" {
						return err
					}
					a.logf("Apple refused while the build is processing (%v); waiting for VALID and trying again.", err)
					if found, err = w.poll(true); err != nil {
						return err
					}
					if err := setWhatToTest(a, found.ID, locale, notes); err != nil {
						return err
					}
				}
				a.logf("Set What to Test (%s) on %s (%s).", locale, version, build)
			}

			result := waitResult{Version: version, Build: build, ID: found.ID, State: processingState(*found)}
			return a.emit(result, result.State)
		},
	}

	app.addFlags(cmd)
	cmd.Flags().StringVar(&until, "until", "valid", "exists or valid")
	cmd.Flags().StringVar(&whatToTest, "what-to-test", "", "a file to set as the build's TestFlight \"What to Test\"")
	cmd.Flags().StringVar(&locale, "locale", "en-US", "the locale of the \"What to Test\" note")
	cmd.Flags().DurationVar(&timeout, "timeout", 40*time.Minute, "give up after this long")
	cmd.Flags().DurationVar(&interval, "interval", time.Minute, "how often to ask")
	return cmd
}

type waiter struct {
	a        *app
	appID    string
	platform string
	version  string
	build    string
	deadline time.Time
	interval time.Duration
}

func processingState(build models.Build) string {
	if build.Attributes.ProcessingState == nil {
		return ""
	}
	return *build.Attributes.ProcessingState
}

// poll returns the build once it exists, or once it is VALID when valid is
// set. FAILED and INVALID end the wait at once: Apple will not change its mind.
func (w *waiter) poll(valid bool) (*models.Build, error) {
	label := fmt.Sprintf("%s (%s)", w.version, w.build)
	for {
		client, err := w.a.api()
		if err != nil {
			return nil, err
		}
		builds, err := client.GetBuilds(w.appID, w.platform, w.version, w.build)
		if err != nil {
			return nil, err
		}

		current := "not there yet"
		if len(builds) > 0 {
			found := builds[0]
			current = processingState(found)
			switch current {
			case "VALID":
				w.a.logf("%s is VALID.", label)
				return &found, nil
			case "FAILED", "INVALID":
				return nil, fmt.Errorf("apple marked %s %s; check the email App Store Connect sent", label, current)
			}
			if !valid {
				w.a.logf("%s exists, %s.", label, current)
				return &found, nil
			}
		}

		if !w.a.now().Before(w.deadline) {
			return nil, fmt.Errorf("gave up waiting; %s is %s", label, current)
		}
		w.a.logf("%s %s: %s", w.a.now().UTC().Format("15:04:05"), label, current)
		w.a.sleep(w.interval)
	}
}

func setWhatToTest(a *app, buildID, locale, notes string) error {
	client, err := a.api()
	if err != nil {
		return err
	}

	localizations, err := client.GetBetaBuildLocalizations(buildID)
	if err != nil {
		return err
	}
	for _, localization := range localizations {
		if localization.Attributes.Locale == locale {
			_, err := client.UpdateBetaBuildLocalization(localization.ID, &notes, nil)
			return err
		}
	}

	_, err = client.CreateBetaBuildLocalization(buildID, models.BetaBuildLocalizationCreateAttributes{
		Locale:   locale,
		WhatsNew: &notes,
	}, nil)
	return err
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
