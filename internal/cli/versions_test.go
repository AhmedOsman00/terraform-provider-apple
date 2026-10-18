// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersionsTreatsMissingPartsAsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"1.2", "1.2.0", 0},
		{"1.10.0", "1.9.9", 1},
		{"2", "1.99.99", 1},
		{"1.0.1", "1.1", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNextVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		newest       string
		year, launch int
		want         string
		description  string
	}{
		{"1.2.0", 2026, 0, "1.3.0", "the next minor by default"},
		{"1.2", 2030, 0, "1.3.0", "years count for nothing without --yearly-major-from"},
		{"1.2.0", 2026, 2026, "1.3.0", "the launch year keeps the major"},
		{"1.3.0", 2027, 2026, "2.0.0", "the first release of the next year takes a major"},
		{"2.0.0", 2027, 2026, "2.1.0", "the rest of that year are minors"},
		{"1.4.0", 2029, 2026, "4.0.0", "a gap of years jumps to that year's major"},
	}
	for _, c := range cases {
		if got := nextVersion(c.newest, c.year, c.launch); got != c.want {
			t.Errorf("%s: nextVersion(%s, %d, %d) = %s, want %s", c.description, c.newest, c.year, c.launch, got, c.want)
		}
	}
}

func TestHighestBuildRefusesWhatIsNotAWholeNumber(t *testing.T) {
	t.Parallel()

	got, err := highestBuild([]string{"3", "12", "9"})
	if err != nil || got != 12 {
		t.Fatalf("highestBuild = %d, %v; want 12", got, err)
	}
	if got, err := highestBuild(nil); err != nil || got != 0 {
		t.Fatalf("highestBuild(nil) = %d, %v; want 0", got, err)
	}
	if _, err := highestBuild([]string{"3", "1.2.3"}); err == nil {
		t.Fatal("highestBuild accepted 1.2.3")
	}
}

const pbxprojFixture = `
		buildSettings = {
			CURRENT_PROJECT_VERSION = 7;
			MARKETING_VERSION = 1.2.0;
		};
		buildSettings = {
			CURRENT_PROJECT_VERSION = 7;
			MARKETING_VERSION = 1.2.0;
		};
`

func TestProjectVersionsReadAndSet(t *testing.T) {
	t.Parallel()

	project := filepath.Join(t.TempDir(), "App.xcodeproj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte(pbxprojFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	version, build, err := projectVersions(project)
	if err != nil || version != "1.2.0" || build != "7" {
		t.Fatalf("projectVersions = %s %s %v; want 1.2.0 7", version, build, err)
	}

	count, err := setProjectVersions(project, "1.3.0", "1")
	if err != nil || count != 2 {
		t.Fatalf("setProjectVersions = %d, %v; want 2 configurations", count, err)
	}
	version, build, _ = projectVersions(filepath.Join(project, "project.pbxproj"))
	if version != "1.3.0" || build != "1" {
		t.Fatalf("after set: %s %s; want 1.3.0 1", version, build)
	}
}
