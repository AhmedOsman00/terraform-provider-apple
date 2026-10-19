// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/spf13/cobra"
)

// submittedVersionStates are the version states that mean it is already with
// App Review, so submitting again has nothing to do.
var submittedVersionStates = map[string]bool{
	"WAITING_FOR_REVIEW": true,
	"IN_REVIEW":          true,
}

type submitResult struct {
	Version          string `json:"version"`
	SubmissionID     string `json:"submission_id,omitempty"`
	State            string `json:"state"`
	AlreadySubmitted bool   `json:"already_submitted,omitempty"`
}

func newReleaseSubmitCommand(a *app) *cobra.Command {
	var app target

	cmd := &cobra.Command{
		Use:   "submit VERSION",
		Short: "Submit VERSION for App Review",
		Long: "Submits the App Store version VERSION for review, the way \"Add for Review\" and\n" +
			"\"Submit to App Review\" do in App Store Connect. Terraform prepares the version and\n" +
			"attaches its build; submitting is an event rather than state, so it is done here.\n\n" +
			"Before anything is sent it checks that VERSION can be submitted and that its build is\n" +
			"attached and VALID. An open draft submission is reused, since Apple allows one per\n" +
			"platform, and a submission App Review left with unresolved issues is resubmitted rather\n" +
			"than replaced. Apple checks the version when it is added, and every problem it reports is\n" +
			"printed.\n\n" +
			"A version already waiting for or in review is reported, not resubmitted, so a pipeline\n" +
			"can run this again safely.",
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

			var found *models.AppStoreVersion
			for i := range versions {
				if compareVersions(versions[i].Attributes.VersionString, version) == 0 {
					found = &versions[i]
				}
			}
			if found == nil {
				return fmt.Errorf("app has no %s version %s; create it with apple_app_store_version first", app.platform, version)
			}

			current := state(*found)
			switch {
			case submittedVersionStates[current]:
				a.logf("%s is already %s.", version, current)
				return a.emit(submitResult{Version: version, State: current, AlreadySubmitted: true}, current)
			case !editableStates[current] && current != "READY_FOR_REVIEW":
				return fmt.Errorf("%s is %s, which cannot be submitted", version, orNone(current))
			}

			client, err := a.api()
			if err != nil {
				return err
			}

			build, err := client.GetAppStoreVersionBuild(found.ID)
			if err != nil {
				return err
			}
			if build == nil {
				return fmt.Errorf("%s has no build attached; set build_number on its apple_app_store_version and apply", version)
			}
			if buildState := processingState(*build); buildState != models.BuildProcessingStateValid {
				return fmt.Errorf("the build attached to %s is %s, and only a VALID build can be submitted; "+
					"wait for it with apple build wait", version, orNone(buildState))
			}
			if build.Attributes.UsesNonExemptEncryption == nil {
				a.logf("  ! The build has no export compliance answer. App Store Connect will hold the version " +
					"in WAITING_FOR_EXPORT_COMPLIANCE until it has one; set ITSAppUsesNonExemptEncryption in Info.plist.")
			}

			submission, err := openSubmission(a, appID, app.platform)
			if err != nil {
				return err
			}

			items, err := client.GetReviewSubmissionItems(submission.ID)
			if err != nil {
				return err
			}
			included := false
			for _, item := range items {
				if item.VersionID() == found.ID {
					included = true
				}
			}
			if others := len(items) - boolCount(included); others > 0 {
				a.logf("  ! The submission already holds %d other item(s), which go to review with %s.", others, version)
			}
			if !included {
				if _, err := client.AddVersionToReviewSubmission(submission.ID, found.ID); err != nil {
					return fmt.Errorf("apple will not take %s for review:\n  %s", version, strings.ReplaceAll(err.Error(), "; ", "\n  "))
				}
			}

			submitted, err := client.SubmitReviewSubmission(submission.ID)
			if err != nil {
				return fmt.Errorf("could not submit %s: %w", version, err)
			}

			a.logf("Submitted %s for review.", version)
			result := submitResult{Version: version, SubmissionID: submitted.ID, State: submitted.Attributes.State}
			return a.emit(result, orNone(result.State))
		},
	}

	app.addFlags(cmd)
	return cmd
}

// openSubmission returns the draft submission to add a version to: one App
// Review left with unresolved issues first, since a fix is resubmitted through
// it, then an open draft, and only then a new one. A submission already with
// App Review is something else in review, and blocks this one.
func openSubmission(a *app, appID, platform string) (*models.ReviewSubmission, error) {
	client, err := a.api()
	if err != nil {
		return nil, err
	}

	submissions, err := client.GetReviewSubmissions(appID, platform, []string{
		models.ReviewSubmissionStateUnresolvedIssues,
		models.ReviewSubmissionStateReadyForReview,
		models.ReviewSubmissionStateWaitingForReview,
		models.ReviewSubmissionStateInReview,
	})
	if err != nil {
		return nil, err
	}

	byState := map[string]*models.ReviewSubmission{}
	for i := range submissions {
		byState[submissions[i].Attributes.State] = &submissions[i]
	}

	for _, blocking := range []string{models.ReviewSubmissionStateWaitingForReview, models.ReviewSubmissionStateInReview} {
		if submission := byState[blocking]; submission != nil {
			return nil, fmt.Errorf("another submission (%s) is %s for %s; wait for it, or cancel it in App Store Connect",
				submission.ID, blocking, platform)
		}
	}
	for _, reusable := range []string{models.ReviewSubmissionStateUnresolvedIssues, models.ReviewSubmissionStateReadyForReview} {
		if submission := byState[reusable]; submission != nil {
			a.logf("Using the %s submission %s.", reusable, submission.ID)
			return submission, nil
		}
	}

	return client.CreateReviewSubmission(appID, platform)
}

func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}
