// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// appleIncompleteDeclaration is Apple's refusal, verbatim but for line
// breaks, of a write to a never-answered declaration that answered the
// original questionnaire alone (November 2026).
const appleIncompleteDeclaration = "API error (status 409): The provided entity is missing a required attribute - " +
	"You must provide a value for the attribute 'parentalControls' with this request; The provided entity is " +
	"missing a required attribute - You must provide a value for the attribute 'healthOrWellnessTopics' with " +
	"this request; The provided entity is missing a required attribute - You must provide a value for the " +
	"attribute 'violenceRealisticProlongedGraphicOrSadistic' with this request; The provided entity is missing " +
	"a required attribute - You must provide a value for the attribute 'ageAssurance' with this request; The " +
	"provided entity is missing a required attribute - You must provide a value for the attribute " +
	"'advertising' with this request; The provided entity is missing a required attribute - You must provide " +
	"a value for the attribute 'messagingAndChat' with this request; The provided entity is missing a " +
	"required attribute - You must provide a value for the attribute 'socialMedia' with this request; The " +
	"provided entity is missing a required attribute - You must provide a value for the attribute " +
	"'gunsOrOtherWeapons' with this request; The provided entity is missing a required attribute - You must " +
	"provide a value for the attribute 'lootBox' with this request"

func TestMissingAgeRatingAnswers(t *testing.T) {
	modeled, unmodeled := missingAgeRatingAnswers(appleIncompleteDeclaration)

	want := []string{
		"advertising", "age_assurance", "guns_or_other_weapons", "health_or_wellness_topics", "loot_box",
		"messaging_and_chat", "parental_controls", "social_media",
		"violence_realistic_prolonged_graphic_or_sadistic",
	}
	if !reflect.DeepEqual(modeled, want) {
		t.Errorf("modeled = %v, want %v", modeled, want)
	}
	if len(unmodeled) != 0 {
		t.Errorf("unmodeled = %v, want none", unmodeled)
	}
}

// TestMissingAgeRatingAnswersNamesUnmodeledQuestions covers a question Apple
// added before the provider did: Apple already reports
// gracRatingClassificationNumber, which the provider has no attribute for.
func TestMissingAgeRatingAnswersNamesUnmodeledQuestions(t *testing.T) {
	msg := "You must provide a value for the attribute 'gracRatingClassificationNumber' with this request; " +
		"You must provide a value for the attribute 'lootBox' with this request; " +
		"You must provide a value for the attribute 'lootBox' with this request"

	modeled, unmodeled := missingAgeRatingAnswers(msg)
	if !reflect.DeepEqual(modeled, []string{"loot_box"}) {
		t.Errorf("modeled = %v, want [loot_box] once", modeled)
	}
	if !reflect.DeepEqual(unmodeled, []string{"gracRatingClassificationNumber"}) {
		t.Errorf("unmodeled = %v, want [gracRatingClassificationNumber]", unmodeled)
	}

	detail := incompleteAgeRatingDetail("123", msg)
	for _, s := range []string{"loot_box", "gracRatingClassificationNumber", "App Store Connect"} {
		if !strings.Contains(detail, s) {
			t.Errorf("detail does not mention %q:\n%s", s, detail)
		}
	}
}

// TestAgeRatingAttributeNamesMatchTheSchema checks that every wire attribute
// translates to an attribute the resource has, so the diagnostic never tells
// anyone to set an attribute that does not exist.
func TestAgeRatingAttributeNamesMatchTheSchema(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewAgeRatingDeclarationResource().Schema(context.Background(), resource.SchemaRequest{}, resp)

	for wire, name := range ageRatingAttributeNames {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("%s translates to %s, which the schema does not have", wire, name)
		}
	}
	if len(ageRatingAttributeNames) == 0 {
		t.Fatal("no wire attributes were read from the model")
	}
}
