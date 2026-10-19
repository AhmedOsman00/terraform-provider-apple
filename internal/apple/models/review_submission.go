// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Review submission states.
//
// A submission is READY_FOR_REVIEW while it is a draft that items can be added
// to, and UNRESOLVED_ISSUES after App Review rejected something in it -- which
// is the submission a fix is resubmitted through, not a new one. Apple holds at
// most one open submission per app and platform.
const (
	ReviewSubmissionStateReadyForReview   = "READY_FOR_REVIEW"
	ReviewSubmissionStateWaitingForReview = "WAITING_FOR_REVIEW"
	ReviewSubmissionStateInReview         = "IN_REVIEW"
	ReviewSubmissionStateUnresolvedIssues = "UNRESOLVED_ISSUES"
	ReviewSubmissionStateCanceling        = "CANCELING"
	ReviewSubmissionStateCompleting       = "COMPLETING"
	ReviewSubmissionStateComplete         = "COMPLETE"
)

// ReviewSubmission is one submission to App Review: a container for the
// version, and anything else, that is reviewed together.
type ReviewSubmission struct {
	Type       string                     `json:"type"`
	ID         string                     `json:"id"`
	Attributes ReviewSubmissionAttributes `json:"attributes"`
}

type ReviewSubmissionAttributes struct {
	Platform      string  `json:"platform,omitempty"`
	SubmittedDate *string `json:"submittedDate,omitempty"`
	State         string  `json:"state,omitempty"`
}

type ReviewSubmissionCreateRequest struct {
	Type          string                        `json:"type"`
	Attributes    ReviewSubmissionAttributes    `json:"attributes"`
	Relationships map[string]ResourceIdentifier `json:"relationships"`
}

// ReviewSubmissionSubmitRequest sends a draft submission to App Review. Only
// submitted is set: canceled and platform are left out rather than sent false
// or empty.
type ReviewSubmissionSubmitRequest struct {
	Type       string                           `json:"type"`
	ID         string                           `json:"id"`
	Attributes ReviewSubmissionSubmitAttributes `json:"attributes"`
}

type ReviewSubmissionSubmitAttributes struct {
	Submitted bool `json:"submitted"`
}

// ReviewSubmissionItem is one thing in a submission. Only an App Store version
// is modelled; Apple also accepts custom product pages, experiments and in-app
// events.
type ReviewSubmissionItem struct {
	Type          string                             `json:"type"`
	ID            string                             `json:"id"`
	Attributes    ReviewSubmissionItemAttributes     `json:"attributes"`
	Relationships *ReviewSubmissionItemRelationships `json:"relationships,omitempty"`
}

type ReviewSubmissionItemAttributes struct {
	State string `json:"state,omitempty"`
}

type ReviewSubmissionItemRelationships struct {
	AppStoreVersion *NullableResourceIdentifier `json:"appStoreVersion,omitempty"`
}

// VersionID is the App Store version the item carries, or "" for another kind
// of item.
func (i ReviewSubmissionItem) VersionID() string {
	if i.Relationships == nil || i.Relationships.AppStoreVersion == nil || i.Relationships.AppStoreVersion.Data == nil {
		return ""
	}
	return i.Relationships.AppStoreVersion.Data.ID
}

type ReviewSubmissionItemCreateRequest struct {
	Type          string                        `json:"type"`
	Relationships map[string]ResourceIdentifier `json:"relationships"`
}
