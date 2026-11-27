// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"
	"slices"
	"strings"
)

// Part is a piece of the account that can be exported on its own, named the
// way --only takes it.
type Part string

const (
	PartListing      Part = "listing"
	PartProducts     Part = "products"
	PartTestFlight   Part = "testflight"
	PartBundleIDs    Part = "bundle-ids"
	PartProfiles     Part = "profiles"
	PartCertificates Part = "certificates"
	PartDevices      Part = "devices"
	PartMerchantIDs  Part = "merchant-ids"
	PartPassTypeIDs  Part = "pass-type-ids"
	PartUsers        Part = "users"
	PartInvitations  Part = "invitations"
)

// Scope says what a part is exported for under --app.
type Scope string

const (
	// ScopeApp parts belong to an app, and --app exports the named apps' own.
	ScopeApp Scope = "app"
	// ScopeReferenced parts are exported only as far as the app's profiles
	// use them.
	ScopeReferenced Scope = "referenced"
	// ScopeAccount parts belong to the whole account, and --app leaves them
	// out unless --only names them.
	ScopeAccount Scope = "account"
)

// PartInfo describes a part: its scope, and the block types it can write.
type PartInfo struct {
	Part  Part     `json:"part"`
	Scope Scope    `json:"scope"`
	Types []string `json:"types"`
}

// Parts lists every part, in the order they are exported. Types is the
// complete list of what each part can write, data sources included;
// TestEveryTypeHasAPart keeps it in step with the exporters.
var Parts = []PartInfo{
	{PartBundleIDs, ScopeApp, []string{"apple_bundle_id", "apple_bundle_id_capability"}},
	{PartCertificates, ScopeReferenced, []string{"data.apple_certificates"}},
	{PartDevices, ScopeReferenced, []string{"apple_device", "data.apple_devices"}},
	{PartMerchantIDs, ScopeAccount, []string{"apple_merchant_id"}},
	{PartPassTypeIDs, ScopeAccount, []string{"apple_pass_type_id"}},
	{PartProfiles, ScopeApp, []string{"apple_profile"}},
	{PartListing, ScopeApp, []string{
		"apple_app_settings", "apple_app_info", "apple_app_info_localization",
		"apple_app_age_rating_declaration", "apple_app_store_version",
		"apple_app_store_version_localization", "apple_app_store_review_detail",
		"apple_app_price_schedule", "apple_app_availability",
	}},
	{PartProducts, ScopeApp, []string{
		"apple_subscription_group", "apple_subscription_group_localization",
		"apple_subscription", "apple_subscription_localization",
		"apple_subscription_availability", "apple_subscription_price_schedule",
		"apple_subscription_introductory_offer_schedule",
		"apple_in_app_purchase", "apple_in_app_purchase_localization",
		"apple_in_app_purchase_price_schedule", "apple_in_app_purchase_availability",
	}},
	{PartTestFlight, ScopeApp, []string{
		"apple_beta_group", "apple_beta_tester", "apple_beta_app_localization",
		"apple_beta_app_review_detail",
	}},
	{PartUsers, ScopeAccount, []string{"apple_user"}},
	{PartInvitations, ScopeAccount, []string{"apple_user_invitation"}},
}

// aliases are the names --only took before it had parts, kept so existing
// commands keep working.
var aliases = map[string][]Part{
	"signing": {PartBundleIDs, PartProfiles, PartCertificates, PartDevices, PartMerchantIDs, PartPassTypeIDs},
	"team":    {PartUsers, PartInvitations},
	"app":     {PartListing},
}

// ParsePart accepts a part or an alias as --only takes it, and returns the
// parts it stands for.
func ParsePart(name string) ([]Part, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if parts, ok := aliases[name]; ok {
		return slices.Clone(parts), nil
	}
	for _, info := range Parts {
		if string(info.Part) == name {
			return []Part{info.Part}, nil
		}
	}
	return nil, fmt.Errorf("unknown part %q: expected one of %s, or signing, team or app", name, strings.Join(partNames(), ", "))
}

func partNames() []string {
	names := make([]string, 0, len(Parts))
	for _, info := range Parts {
		names = append(names, string(info.Part))
	}
	return names
}

// partOf is the part a block type belongs to.
func partOf(blockType string) Part {
	for _, info := range Parts {
		if slices.Contains(info.Types, blockType) {
			return info.Part
		}
	}
	return ""
}

// DescribeParts is the table --help prints: every part, its scope, what it
// does under --app, and the block types it can write.
func DescribeParts() string {
	under := map[Scope]string{
		ScopeApp:        "the named apps' own",
		ScopeReferenced: "only what their profiles use",
		ScopeAccount:    "only when --only names it",
	}
	var b strings.Builder
	for _, info := range Parts {
		scope := under[info.Scope]
		if info.Part == PartDevices {
			// Named, devices are exported as the account's, not as referenced.
			scope += ", as data sources;\n" + strings.Repeat(" ", 29) + "every device, as resources, when --only names it"
		}
		fmt.Fprintf(&b, "  %-14s under --app: %s\n", info.Part, scope)
		line := ""
		for i, name := range info.Types {
			if i < len(info.Types)-1 {
				name += ","
			}
			if line != "" && len(line)+1+len(name) > 72 {
				fmt.Fprintf(&b, "  %-14s %s\n", "", line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += name
		}
		fmt.Fprintf(&b, "  %-14s %s\n", "", line)
	}
	b.WriteString("  Aliases: signing = bundle-ids, profiles, certificates, devices, merchant-ids,\n")
	b.WriteString("  pass-type-ids; team = users, invitations; app = listing.\n")
	return b.String()
}
