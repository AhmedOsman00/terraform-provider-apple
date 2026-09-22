// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// UserFilters is the filtering configuration for team members.
//
// Apple's own filters on /v1/users are not used: the collection is a team's
// worth of people, so it is read whole and narrowed here, which keeps the
// pattern filters expressible at all -- filter[username] is an exact match.
type UserFilters struct {
	UsernamePattern     string
	NamePattern         string
	Roles               []string
	AllAppsVisible      *bool
	ProvisioningAllowed *bool
}

// FilterUsers applies filters to a list of team members.
func FilterUsers(users []models.User, filters UserFilters) ([]models.User, error) {
	// Reject malformed patterns up front rather than silently matching nothing.
	usernameRE, err := compilePattern("username_pattern", filters.UsernamePattern)
	if err != nil {
		return nil, err
	}

	nameRE, err := compilePattern("name_pattern", filters.NamePattern)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.User, 0, len(users))
	for _, user := range users {
		if shouldIncludeUser(user, filters, usernameRE, nameRE) {
			filtered = append(filtered, user)
		}
	}

	return filtered, nil
}

// compilePattern compiles one case-insensitive filter pattern, naming the
// attribute in the error so a bad regex says which one it was.
func compilePattern(attribute, pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}

	compiled, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", attribute, pattern, err)
	}

	return compiled, nil
}

// shouldIncludeUser reports whether one member passes every filter.
//
// The role filter is an any-of rather than an exact set: asking for DEVELOPER
// means "people who can develop", and most of them carry other roles too.
func shouldIncludeUser(user models.User, filters UserFilters, usernameRE, nameRE *regexp.Regexp) bool {
	if usernameRE != nil && !usernameRE.MatchString(username(user)) {
		return false
	}

	if nameRE != nil && !nameRE.MatchString(fullName(user)) {
		return false
	}

	if len(filters.Roles) > 0 && !hasAnyRole(user, filters.Roles) {
		return false
	}

	if filters.AllAppsVisible != nil && boolValue(user.Attributes.AllAppsVisible) != *filters.AllAppsVisible {
		return false
	}

	if filters.ProvisioningAllowed != nil && boolValue(user.Attributes.ProvisioningAllowed) != *filters.ProvisioningAllowed {
		return false
	}

	return true
}

// hasAnyRole reports whether a member holds at least one of the named roles.
func hasAnyRole(user models.User, wanted []string) bool {
	for _, role := range user.Attributes.Roles {
		for _, want := range wanted {
			if role == want {
				return true
			}
		}
	}

	return false
}

// SortUsers sorts team members by the specified field and order.
//
// Sorting is case-insensitive and falls back to the username, which is the one
// attribute every member has: a person whose Apple Account reports no name
// would otherwise sort arbitrarily against every other one.
func SortUsers(users []models.User, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.SliceStable(users, func(i, j int) bool {
		var left, right string

		switch sortBy {
		case "first_name":
			left, right = stringValue(users[i].Attributes.FirstName), stringValue(users[j].Attributes.FirstName)
		case "last_name":
			left, right = stringValue(users[i].Attributes.LastName), stringValue(users[j].Attributes.LastName)
		default:
			left, right = username(users[i]), username(users[j])
		}

		left, right = strings.ToLower(left), strings.ToLower(right)
		if left == right {
			left, right = strings.ToLower(username(users[i])), strings.ToLower(username(users[j]))
		}

		if ascending {
			return left < right
		}

		return left > right
	})
}

// LimitUsers caps the number of members returned.
func LimitUsers(users []models.User, limit int) []models.User {
	if limit <= 0 || limit >= len(users) {
		return users
	}

	return users[:limit]
}

// username reports the address a member signs in with, or the empty string when
// Apple omits it.
func username(user models.User) string {
	return stringValue(user.Attributes.Username)
}

// fullName joins the names Apple holds for a member, so name_pattern can match
// across both without a configuration having to know which half it is in.
func fullName(user models.User) string {
	return strings.TrimSpace(
		stringValue(user.Attributes.FirstName) + " " + stringValue(user.Attributes.LastName))
}

// stringValue dereferences an optional API string, mapping a missing value to
// the empty string.
func stringValue(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

// boolValue dereferences an optional API bool. A missing value is false, which
// is what Apple means by omitting one of these: a member Apple does not report
// as seeing every app does not see every app.
func boolValue(b *bool) bool {
	return b != nil && *b
}
