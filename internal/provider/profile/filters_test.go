package profile

import (
	"context"
	"testing"

	"terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(values ...string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

func statePtr(s models.ProfileState) *models.ProfileState { return &s }
func typePtr(p models.ProfileType) *models.ProfileType    { return &p }

func profiles() []models.Profile {
	return []models.Profile{
		{ID: "1", Attributes: models.ProfileAttributes{
			Name: "App Store Profile", Platform: models.ProfilePlatform("IOS"),
			ProfileState: statePtr(models.ProfileState("ACTIVE")),
			ProfileType:  typePtr(models.ProfileType("IOS_APP_STORE")),
		}},
		{ID: "2", Attributes: models.ProfileAttributes{
			Name: "development profile", Platform: models.ProfilePlatform("IOS"),
			ProfileState: statePtr(models.ProfileState("EXPIRED")),
			ProfileType:  typePtr(models.ProfileType("IOS_APP_DEVELOPMENT")),
		}},
		{ID: "3", Attributes: models.ProfileAttributes{
			Name: "Mac Profile", Platform: models.ProfilePlatform("MAC_OS"),
			// State and type omitted: Apple does not always report them.
		}},
	}
}

func names(list []models.Profile) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Attributes.Name)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func baseConfig() profilesDataSourceModel {
	return profilesDataSourceModel{
		Platform:     types.StringNull(),
		Platforms:    types.ListNull(types.StringType),
		NamePattern:  types.StringNull(),
		ProfileState: types.StringNull(),
		ProfileType:  types.StringNull(),
		BundleID:     types.StringNull(),
		Limit:        types.Int64Null(),
		SortBy:       types.StringNull(),
		SortOrder:    types.StringNull(),
	}
}

func TestApplyFilters(t *testing.T) {
	ctx := context.Background()
	d := &profilesDataSource{}

	tests := []struct {
		name   string
		mutate func(*profilesDataSourceModel)
		want   []string
	}{
		{name: "no filters", mutate: func(c *profilesDataSourceModel) {}, want: []string{"App Store Profile", "development profile", "Mac Profile"}},
		{name: "single platform", mutate: func(c *profilesDataSourceModel) { c.Platform = types.StringValue("MAC_OS") }, want: []string{"Mac Profile"}},
		{name: "platforms list", mutate: func(c *profilesDataSourceModel) { c.Platforms = strList("IOS") }, want: []string{"App Store Profile", "development profile"}},
		{name: "name regex", mutate: func(c *profilesDataSourceModel) { c.NamePattern = types.StringValue("^App") }, want: []string{"App Store Profile"}},
		{name: "profile state", mutate: func(c *profilesDataSourceModel) { c.ProfileState = types.StringValue("ACTIVE") }, want: []string{"App Store Profile"}},
		// A profile with no state reported must not match a state filter.
		{name: "state filter excludes profiles Apple left unset", mutate: func(c *profilesDataSourceModel) { c.ProfileState = types.StringValue("EXPIRED") }, want: []string{"development profile"}},
		{name: "profile type", mutate: func(c *profilesDataSourceModel) { c.ProfileType = types.StringValue("IOS_APP_STORE") }, want: []string{"App Store Profile"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			tt.mutate(&config)

			got, err := d.applyFilters(ctx, profiles(), config)
			if err != nil {
				t.Fatalf("applyFilters() returned error: %v", err)
			}
			if !equalStrings(names(got), tt.want) {
				t.Errorf("applyFilters() = %v, want %v", names(got), tt.want)
			}
		})
	}
}

func TestApplyFiltersRejectsBadNamePattern(t *testing.T) {
	d := &profilesDataSource{}
	config := baseConfig()
	config.NamePattern = types.StringValue("(unclosed")

	got, err := d.applyFilters(context.Background(), profiles(), config)
	if err == nil {
		t.Fatalf("applyFilters() = %v with nil error, want an error naming the bad pattern", names(got))
	}
	if got != nil {
		t.Errorf("applyFilters() returned %v alongside an error, want nil", names(got))
	}
}

func TestApplySorting(t *testing.T) {
	ctx := context.Background()
	d := &profilesDataSource{}

	for _, tt := range []struct {
		name      string
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"name ascending is case-insensitive", "name", "asc", []string{"App Store Profile", "development profile", "Mac Profile"}},
		{"name descending", "name", "desc", []string{"Mac Profile", "development profile", "App Store Profile"}},
		{"platform ascending", "platform", "asc", []string{"App Store Profile", "development profile", "Mac Profile"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			config.SortBy = types.StringValue(tt.sortBy)
			config.SortOrder = types.StringValue(tt.sortOrder)

			got := d.applySorting(ctx, profiles(), config)
			if !equalStrings(names(got), tt.want) {
				t.Errorf("applySorting(%q, %q) = %v, want %v", tt.sortBy, tt.sortOrder, names(got), tt.want)
			}
		})
	}
}

func TestApplyLimit(t *testing.T) {
	ctx := context.Background()
	d := &profilesDataSource{}

	for _, tt := range []struct {
		name  string
		limit types.Int64
		want  int
	}{
		{"null means no limit", types.Int64Null(), 3},
		{"zero means no limit", types.Int64Value(0), 3},
		{"limit below length truncates", types.Int64Value(2), 2},
		{"limit above length", types.Int64Value(10), 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig()
			config.Limit = tt.limit

			if got := d.applyLimit(ctx, profiles(), config); len(got) != tt.want {
				t.Errorf("applyLimit(%v) returned %d profiles, want %d", tt.limit, len(got), tt.want)
			}
		})
	}
}
