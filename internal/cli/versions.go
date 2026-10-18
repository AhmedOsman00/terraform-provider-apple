// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)

// releasedStates are the version states Apple has finished with. A version in
// one of them is part of the app's history and cannot be renamed.
var releasedStates = map[string]bool{
	"READY_FOR_DISTRIBUTION":      true,
	"PROCESSING_FOR_DISTRIBUTION": true,
	"REPLACED_WITH_NEW_VERSION":   true,
	"PENDING_DEVELOPER_RELEASE":   true,
	"PENDING_APPLE_RELEASE":       true,
	"ACCEPTED":                    true,
}

// editableStates are the ones in which Apple still accepts changes to a
// version, its number included.
var editableStates = map[string]bool{
	"PREPARE_FOR_SUBMISSION": true,
	"DEVELOPER_REJECTED":     true,
	"REJECTED":               true,
	"METADATA_REJECTED":      true,
	"INVALID_BINARY":         true,
}

func checkVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return usageError("Apple accepts up to three dot-separated numbers, such as 1.1, 1.1.0 or 2.14.3; got %q", version)
	}
	return nil
}

// versionParts reads a version as three numbers, so 1.2 and 1.2.0 compare
// equal the way Apple treats them.
func versionParts(version string) [3]int {
	var parts [3]int
	for i, field := range strings.SplitN(version, ".", 3) {
		parts[i], _ = strconv.Atoi(field)
	}
	return parts
}

func compareVersions(a, b string) int {
	x, y := versionParts(a), versionParts(b)
	for i := range x {
		switch {
		case x[i] > y[i]:
			return 1
		case x[i] < y[i]:
			return -1
		}
	}
	return 0
}

// nextVersion is the minor after newest. With yearlyMajorFrom set, the first
// release of a calendar year takes the next major instead, the major being the
// number of years since that one plus one: 1.3.0 is followed by 2.0.0 in the
// second year.
func nextVersion(newest string, year, yearlyMajorFrom int) string {
	parts := versionParts(newest)
	if yearlyMajorFrom > 0 {
		if major := year - yearlyMajorFrom + 1; parts[0] < major {
			return fmt.Sprintf("%d.0.0", major)
		}
	}
	return fmt.Sprintf("%d.%d.0", parts[0], parts[1]+1)
}

// highestBuild is the largest integer build number, refusing any that is not
// one, since the next build has to be worked out from it.
func highestBuild(numbers []string) (int, error) {
	highest := 0
	var odd []string
	for _, number := range numbers {
		n, err := strconv.Atoi(number)
		if err != nil {
			odd = append(odd, number)
			continue
		}
		highest = max(highest, n)
	}
	if len(odd) > 0 {
		return 0, fmt.Errorf("these build numbers are not whole numbers, so the next one cannot be worked out: %s",
			strings.Join(odd, ", "))
	}
	return highest, nil
}
