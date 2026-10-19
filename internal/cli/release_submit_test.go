// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"strings"
	"testing"
)

const validBuild = `{"type":"builds","id":"B-7","attributes":{"version":"7","processingState":"VALID","usesNonExemptEncryption":false}}`

func TestReleaseSubmitCreatesASubmissionAndSubmitsIt(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{
		versions:      map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION", "1.1.0": "PREPARE_FOR_SUBMISSION"},
		attachedBuild: validBuild,
	}
	h := newHarness(t, fake)
	if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if fake.created != 1 {
		t.Errorf("created %d submissions, want 1", fake.created)
	}
	if strings.Join(fake.added, ",") != "RS-NEW V-1.1.0" || strings.Join(fake.submitted, ",") != "RS-NEW" {
		t.Errorf("added %v, submitted %v", fake.added, fake.submitted)
	}
	if got := h.out.String(); got != "WAITING_FOR_REVIEW\n" {
		t.Errorf("stdout = %q", got)
	}
}

// TestReleaseSubmitResubmitsUnresolvedIssues covers a rejection: the fix goes
// back through the submission App Review left open, which already holds the
// version, rather than a new one.
func TestReleaseSubmitResubmitsUnresolvedIssues(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{
		versions:      map[string]string{"1.1.0": "REJECTED"},
		attachedBuild: validBuild,
		submissions: []string{
			`{"type":"reviewSubmissions","id":"RS-DRAFT","attributes":{"state":"READY_FOR_REVIEW"}}`,
			`{"type":"reviewSubmissions","id":"RS-REJECTED","attributes":{"state":"UNRESOLVED_ISSUES"}}`,
		},
		items: map[string][]string{"RS-REJECTED": {
			`{"type":"reviewSubmissionItems","id":"I1","attributes":{"state":"REJECTED"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"V-1.1.0"}}}}`,
		}},
	}
	h := newHarness(t, fake)
	if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if fake.created != 0 || len(fake.added) != 0 {
		t.Errorf("created %d and added %v; the rejected submission already held the version", fake.created, fake.added)
	}
	if strings.Join(fake.submitted, ",") != "RS-REJECTED" {
		t.Errorf("submitted %v, want RS-REJECTED", fake.submitted)
	}
}

func TestReleaseSubmitReportsEveryProblemApplePrints(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{
		versions:      map[string]string{"1.1.0": "PREPARE_FOR_SUBMISSION"},
		attachedBuild: validBuild,
		itemErrors: []string{
			`{"status":"409","code":"STATE_ERROR","title":"Screenshots are missing","detail":"APP_IPHONE_67 has none"}`,
			`{"status":"409","code":"STATE_ERROR","title":"Content rights are missing","detail":"Set them in App Information"}`,
		},
	}
	h := newHarness(t, fake)
	if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	for _, want := range []string{"APP_IPHONE_67 has none", "Set them in App Information"} {
		if !strings.Contains(h.errOut.String(), want) {
			t.Errorf("stderr is missing %q:\n%s", want, h.errOut)
		}
	}
	if len(fake.submitted) != 0 {
		t.Errorf("submitted %v after Apple refused the version", fake.submitted)
	}
}

func TestReleaseSubmitIsANoOpForAVersionInReview(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{versions: map[string]string{"1.1.0": "WAITING_FOR_REVIEW"}}
	h := newHarness(t, fake)
	if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if len(fake.submitted) != 0 || h.out.String() != "WAITING_FOR_REVIEW\n" {
		t.Errorf("submitted %v, stdout %q", fake.submitted, h.out)
	}
}

func TestReleaseSubmitRefusesWhatCannotBeSubmitted(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		fake *fakeConnect
		want string
	}{
		"no build": {
			fake: &fakeConnect{versions: map[string]string{"1.1.0": "PREPARE_FOR_SUBMISSION"}},
			want: "no build attached",
		},
		"processing build": {
			fake: &fakeConnect{
				versions:      map[string]string{"1.1.0": "PREPARE_FOR_SUBMISSION"},
				attachedBuild: `{"type":"builds","id":"B-7","attributes":{"processingState":"PROCESSING"}}`,
			},
			want: "is PROCESSING",
		},
		"released version": {
			fake: &fakeConnect{versions: map[string]string{"1.1.0": "READY_FOR_DISTRIBUTION"}},
			want: "cannot be submitted",
		},
		"missing version": {
			fake: &fakeConnect{versions: map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION"}},
			want: "has no IOS version 1.1.0",
		},
		"another in review": {
			fake: &fakeConnect{
				versions:      map[string]string{"1.1.0": "PREPARE_FOR_SUBMISSION"},
				attachedBuild: validBuild,
				submissions:   []string{`{"type":"reviewSubmissions","id":"RS-OTHER","attributes":{"state":"IN_REVIEW"}}`},
			},
			want: "RS-OTHER) is IN_REVIEW",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t, tc.fake)
			if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if !strings.Contains(h.errOut.String(), tc.want) {
				t.Errorf("stderr is missing %q:\n%s", tc.want, h.errOut)
			}
			if len(tc.fake.submitted) != 0 {
				t.Errorf("submitted %v", tc.fake.submitted)
			}
		})
	}
}

func TestReleaseSubmitWarnsAboutExportCompliance(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{
		versions:      map[string]string{"1.1.0": "PREPARE_FOR_SUBMISSION"},
		attachedBuild: `{"type":"builds","id":"B-7","attributes":{"processingState":"VALID"}}`,
	}
	h := newHarness(t, fake)
	if code := h.run("release", "submit", "1.1.0", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if !strings.Contains(h.errOut.String(), "WAITING_FOR_EXPORT_COMPLIANCE") {
		t.Errorf("no export compliance warning:\n%s", h.errOut)
	}
}
