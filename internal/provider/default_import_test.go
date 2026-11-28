// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// defaultAllowed names attributes of importable resources that may carry a
// Default, keyed "<type>.<attribute path>", with the reason. It is empty: no
// such attribute has needed one yet.
var defaultAllowed = map[string]string{}

// TestImportableResourcesHaveNoDefaults checks that no attribute of a resource
// that can be imported carries a Default, at any depth.
//
// A default plans a change whenever the configuration leaves the attribute
// out, overwriting what Apple holds. That is what an import followed by a
// configuration written by hand looks like, and on an adopting resource it is
// what every create looks like. apple_user.provisioning_allowed defaulting to
// false revoked access on adoption; apple_user_invitation.provisioning_allowed
// forces replacement, so its default would cancel and resend an imported
// invitation. Make the attribute Optional+Computed, keep Apple's value, and say
// in the description what a new record starts with.
func TestImportableResourcesHaveNoDefaults(t *testing.T) {
	ctx := context.Background()

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()
		if _, importable := r.(resource.ResourceWithImportState); !importable {
			continue
		}

		metadata := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, metadata)
		schemaResp := &resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

		for _, name := range attributesWithDefaults("", schemaResp.Schema.Attributes) {
			key := metadata.TypeName + "." + name
			if _, ok := defaultAllowed[key]; ok {
				continue
			}
			t.Errorf("%s has a Default on a resource that can be imported: leaving it out of a "+
				"configuration plans a change over whatever Apple holds. Make it Optional+Computed "+
				"instead, or name it in defaultAllowed with the reason.", key)
		}
	}

	// An exemption that no longer matches anything is stale.
	for key := range defaultAllowed {
		found := false
		for _, newResource := range New("test")().Resources(ctx) {
			r := newResource()
			metadata := &resource.MetadataResponse{}
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, metadata)
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			for _, name := range attributesWithDefaults("", schemaResp.Schema.Attributes) {
				if metadata.TypeName+"."+name == key {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("defaultAllowed names %s, which has no Default any more; remove it", key)
		}
	}
}

// attributesWithDefaults returns the paths of every attribute carrying a
// Default, descending into nested attributes.
//
// It reads the Default and NestedObject fields by reflection because the
// framework gives every attribute kind its own struct and its own Default
// type, and a type switch would silently skip a kind it did not list.
func attributesWithDefaults(prefix string, attributes map[string]schema.Attribute) []string {
	var found []string

	for name, attribute := range attributes {
		attrPath := prefix + name

		value := reflect.ValueOf(attribute)
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			continue
		}

		if field := value.FieldByName("Default"); field.IsValid() && !field.IsZero() {
			found = append(found, attrPath)
		}

		if nested := value.FieldByName("NestedObject"); nested.IsValid() {
			if inner, ok := nested.FieldByName("Attributes").Interface().(map[string]schema.Attribute); ok {
				found = append(found, attributesWithDefaults(attrPath+".", inner)...)
			}
		}
		if inner := value.FieldByName("Attributes"); inner.IsValid() {
			if attrs, ok := inner.Interface().(map[string]schema.Attribute); ok {
				found = append(found, attributesWithDefaults(attrPath+".", attrs)...)
			}
		}
	}

	sort.Strings(found)
	return found
}
