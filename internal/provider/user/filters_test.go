// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// testUser builds one team member for the table below.
func testUser(id, username, firstName, lastName string, roles []string, allApps, provisioning bool) models.User {
	return models.User{
		Type: "users",
		ID:   id,
		Attributes: models.UserAttributes{
			Username:            &username,
			FirstName:           &firstName,
			LastName:            &lastName,
			Roles:               roles,
			AllAppsVisible:      &allApps,
			ProvisioningAllowed: &provisioning,
		},
	}
}

func testTeam() []models.User {
	return []models.User{
		testUser("u1", "ada@example.com", "Ada", "Lovelace", []string{"DEVELOPER", "APP_MANAGER"}, false, true),
		testUser("u2", "grace@example.com", "Grace", "Hopper", []string{"ADMIN"}, true, true),
		testUser("u3", "alan@example.net", "Alan", "Turing", []string{"DEVELOPER"}, true, false),
		testUser("u4", "katherine@example.com", "Katherine", "Johnson", []string{"FINANCE", "ACCESS_TO_REPORTS"}, false, false),
	}
}

func boolPtr(b bool) *bool { return &b }

func TestFilterUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filters UserFilters
		wantIDs []string
	}{
		{
			name:    "no filters returns everybody",
			filters: UserFilters{},
			wantIDs: []string{"u1", "u2", "u3", "u4"},
		},
		{
			name:    "username pattern is an unanchored regex",
			filters: UserFilters{UsernamePattern: `@example\.com$`},
			wantIDs: []string{"u1", "u2", "u4"},
		},
		{
			name:    "username pattern is case-insensitive",
			filters: UserFilters{UsernamePattern: "ADA@"},
			wantIDs: []string{"u1"},
		},
		{
			name:    "name pattern spans both halves of the name",
			filters: UserFilters{NamePattern: "grace hopper"},
			wantIDs: []string{"u2"},
		},
		{
			name:    "role filter is an any-of over the roles held",
			filters: UserFilters{Roles: []string{"DEVELOPER"}},
			wantIDs: []string{"u1", "u3"},
		},
		{
			name:    "several roles widen rather than narrow",
			filters: UserFilters{Roles: []string{"ADMIN", "FINANCE"}},
			wantIDs: []string{"u2", "u4"},
		},
		{
			name:    "all_apps_visible false keeps the restricted members",
			filters: UserFilters{AllAppsVisible: boolPtr(false)},
			wantIDs: []string{"u1", "u4"},
		},
		{
			name:    "provisioning_allowed true keeps the signers",
			filters: UserFilters{ProvisioningAllowed: boolPtr(true)},
			wantIDs: []string{"u1", "u2"},
		},
		{
			name: "filters compose",
			filters: UserFilters{
				Roles:               []string{"DEVELOPER"},
				ProvisioningAllowed: boolPtr(true),
			},
			wantIDs: []string{"u1"},
		},
		{
			name:    "a filter nobody matches returns nothing",
			filters: UserFilters{Roles: []string{"MARKETING"}},
			wantIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FilterUsers(testTeam(), tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d users %v, want %d %v", len(got), ids(got), len(tt.wantIDs), tt.wantIDs)
			}

			for i, want := range tt.wantIDs {
				if got[i].ID != want {
					t.Errorf("user %d = %q, want %q", i, got[i].ID, want)
				}
			}
		})
	}
}

// TestFilterUsersRejectsMalformedPatterns covers the up-front compile: a bad
// regex has to say so rather than silently matching nobody, which reads as an
// empty team.
func TestFilterUsersRejectsMalformedPatterns(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		filters UserFilters
		want    string
	}{
		{"username", UserFilters{UsernamePattern: "["}, "username_pattern"},
		{"name", UserFilters{NamePattern: "("}, "name_pattern"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := FilterUsers(testTeam(), tt.filters)
			if err == nil {
				t.Fatal("expected an error for a malformed pattern")
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Errorf("error = %q, want it to name %s", got, tt.want)
			}
		})
	}
}

// TestFilterUsersTreatsMissingAttributesAsFalse covers what Apple omits: a
// member reported without allAppsVisible is not a member who sees every app.
func TestFilterUsersTreatsMissingAttributesAsFalse(t *testing.T) {
	t.Parallel()

	username := "minimal@example.com"
	sparse := []models.User{{
		Type:       "users",
		ID:         "u9",
		Attributes: models.UserAttributes{Username: &username},
	}}

	got, err := FilterUsers(sparse, UserFilters{AllAppsVisible: boolPtr(false)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d users, want the member Apple reported nothing about", len(got))
	}

	got, err = FilterUsers(sparse, UserFilters{NamePattern: "anything"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Error("a member with no name matched a name pattern")
	}
}

func TestSortUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantIDs   []string
	}{
		{"defaults to username ascending", "", "", []string{"u1", "u3", "u2", "u4"}},
		{"username descending", "username", "desc", []string{"u4", "u2", "u3", "u1"}},
		{"first name ascending", "first_name", "asc", []string{"u1", "u3", "u2", "u4"}},
		{"last name ascending", "last_name", "asc", []string{"u2", "u4", "u1", "u3"}},
		{"last name descending", "last_name", "desc", []string{"u3", "u1", "u4", "u2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			users := testTeam()
			SortUsers(users, tt.sortBy, tt.sortOrder)

			for i, want := range tt.wantIDs {
				if users[i].ID != want {
					t.Fatalf("order = %v, want %v", ids(users), tt.wantIDs)
				}
			}
		})
	}
}

// TestSortUsersBreaksTiesOnUsername covers the fallback: two people with the
// same first name would otherwise sort arbitrarily between runs, which churns a
// plan for a data source that reports them in order.
func TestSortUsersBreaksTiesOnUsername(t *testing.T) {
	t.Parallel()

	users := []models.User{
		testUser("u2", "zoe.ada@example.com", "Ada", "Byron", nil, true, false),
		testUser("u1", "ada@example.com", "Ada", "Lovelace", nil, true, false),
	}

	SortUsers(users, "first_name", "asc")

	if users[0].ID != "u1" {
		t.Errorf("order = %v, want the tie broken on username", ids(users))
	}
}

func TestLimitUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"zero is no limit", 0, 4},
		{"negative is no limit", -1, 4},
		{"a limit below the count truncates", 2, 2},
		{"a limit above the count is harmless", 10, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := LimitUsers(testTeam(), tt.limit); len(got) != tt.want {
				t.Errorf("got %d users, want %d", len(got), tt.want)
			}
		})
	}
}

func ids(users []models.User) []string {
	out := make([]string, 0, len(users))
	for _, user := range users {
		out = append(out, user.ID)
	}

	return out
}
