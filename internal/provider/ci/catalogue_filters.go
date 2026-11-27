// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// catalogueOptions is what a catalogue data source was asked for, beyond the
// records themselves. The zero value keeps every record in Apple's order.
type catalogueOptions struct {
	namePattern *regexp.Regexp
	sortBy      string // a Terraform attribute name, or "" for Apple's order
	descending  bool
	limit       int // 0 for no limit
}

// nameField is the attribute name_pattern matches, as Apple names it.
func (d *catalogueDataSource) nameField() string {
	if d.kind == "scm_repositories" {
		return "repositoryName"
	}
	return "name"
}

// sortFields maps each sort_by value to the attribute it sorts on, as Apple
// names it.
func (d *catalogueDataSource) sortFields() map[string]string {
	switch d.kind {
	case "ci_products":
		return map[string]string{"name": "name", "created_date": "createdDate"}
	case "scm_repositories":
		return map[string]string{"repository_name": "repositoryName", "owner_name": "ownerName", "last_accessed_date": "lastAccessedDate"}
	default:
		return map[string]string{"name": "name", "version": "version"}
	}
}

func (d *catalogueDataSource) sortKeys() []string {
	keys := make([]string, 0)
	for key := range d.sortFields() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// applyCatalogueOptions filters, sorts and limits decoded records. Sorting is
// stable, so records that compare equal keep Apple's order.
func (d *catalogueDataSource) applyCatalogueOptions(records []map[string]any, options catalogueOptions) []map[string]any {
	kept := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if options.namePattern != nil && !options.namePattern.MatchString(stringField(record, d.nameField())) {
			continue
		}
		kept = append(kept, record)
	}

	if field, ok := d.sortFields()[options.sortBy]; ok {
		compare := strings.Compare
		if options.sortBy == "version" {
			compare = compareVersions
		}
		sort.SliceStable(kept, func(i, j int) bool {
			order := compare(stringField(kept[i], field), stringField(kept[j], field))
			if options.descending {
				return order > 0
			}
			return order < 0
		})
	}

	if options.limit > 0 && len(kept) > options.limit {
		kept = kept[:options.limit]
	}
	return kept
}

func stringField(record map[string]any, field string) string {
	if value, ok := record[field].(string); ok {
		return value
	}
	if value, ok := record[field]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

// compareVersions orders dotted version numbers numerically, so 16.10 comes
// after 16.2. A part that is not a number is compared as text.
func compareVersions(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) || i < len(right); i++ {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		ln, lerr := strconv.Atoi(l)
		rn, rerr := strconv.Atoi(r)
		switch {
		case lerr == nil && rerr == nil:
			if ln != rn {
				if ln < rn {
					return -1
				}
				return 1
			}
		case l != r:
			return strings.Compare(l, r)
		}
	}
	return 0
}
