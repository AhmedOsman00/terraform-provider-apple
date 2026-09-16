// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Age rating frequency enumeration.
//
// Apple carries five values where the questionnaire only ever offers three at a
// time: INFREQUENT_OR_MILD and FREQUENT_OR_INTENSE belong to the original
// questionnaire, while INFREQUENT and FREQUENT belong to the revised one Apple
// introduced alongside the 13+ and 16+ bands. Both sets are accepted here
// because an existing declaration may hold either.
const (
	AgeRatingFrequencyNone              = "NONE"
	AgeRatingFrequencyInfrequentOrMild  = "INFREQUENT_OR_MILD"
	AgeRatingFrequencyFrequentOrIntense = "FREQUENT_OR_INTENSE"
	AgeRatingFrequencyInfrequent        = "INFREQUENT"
	AgeRatingFrequencyFrequent          = "FREQUENT"
)

// ValidAgeRatingFrequencies lists the accepted frequency answers.
var ValidAgeRatingFrequencies = []string{
	AgeRatingFrequencyNone,
	AgeRatingFrequencyInfrequentOrMild,
	AgeRatingFrequencyFrequentOrIntense,
	AgeRatingFrequencyInfrequent,
	AgeRatingFrequencyFrequent,
}

// ValidKidsAgeBands lists the Kids Category bands. A null value means the app
// is not in the Kids Category at all, which is the usual case.
var ValidKidsAgeBands = []string{"FIVE_AND_UNDER", "SIX_TO_EIGHT", "NINE_TO_ELEVEN"}

// ValidAgeRatingOverrides lists the original override values.
//
// An override raises the rating Apple calculates from the answers; it can never
// lower it.
var ValidAgeRatingOverrides = []string{"NONE", "NINE_PLUS", "THIRTEEN_PLUS", "SIXTEEN_PLUS", "SEVENTEEN_PLUS", "UNRATED"}

// ValidAgeRatingOverridesV2 lists the revised override values, in which
// SEVENTEEN_PLUS became EIGHTEEN_PLUS.
var ValidAgeRatingOverridesV2 = []string{"NONE", "NINE_PLUS", "THIRTEEN_PLUS", "SIXTEEN_PLUS", "EIGHTEEN_PLUS", "UNRATED"}

// ValidKoreaAgeRatingOverrides lists the Korea-specific override values.
var ValidKoreaAgeRatingOverrides = []string{"NONE", "FIFTEEN_PLUS", "NINETEEN_PLUS"}

// AgeRatingDeclaration is the answered content questionnaire behind an app's
// age rating.
//
// Apple creates exactly one alongside the AppInfo and publishes no POST and no
// DELETE for it: the record always exists and is only ever patched. Its ID is
// reachable through the AppInfo it hangs off.
//
// The attributes fall into three groups. The frequency questions ask how often
// a kind of content appears. The booleans are yes/no facts about what the app
// does -- whether it has a chat, whether it shows ads, whether it sells loot
// boxes. The overrides raise the computed rating.
type AgeRatingDeclaration struct {
	Type       string                         `json:"type"`
	ID         string                         `json:"id"`
	Attributes AgeRatingDeclarationAttributes `json:"attributes"`
	Links      *ResourceLinks                 `json:"links,omitempty"`
}

// AgeRatingDeclarationAttributes is both what Apple reports and what the update
// request accepts -- the two are identical, so one struct serves both.
//
// Every member is a pointer: an omitted answer leaves Apple's existing value
// alone, which is not the same as answering NONE.
type AgeRatingDeclarationAttributes struct {
	// Frequency questions.
	AlcoholTobaccoOrDrugUseOrReferences         *string `json:"alcoholTobaccoOrDrugUseOrReferences,omitempty"`
	Contests                                    *string `json:"contests,omitempty"`
	GamblingSimulated                           *string `json:"gamblingSimulated,omitempty"`
	GunsOrOtherWeapons                          *string `json:"gunsOrOtherWeapons,omitempty"`
	HorrorOrFearThemes                          *string `json:"horrorOrFearThemes,omitempty"`
	MatureOrSuggestiveThemes                    *string `json:"matureOrSuggestiveThemes,omitempty"`
	MedicalOrTreatmentInformation               *string `json:"medicalOrTreatmentInformation,omitempty"`
	ProfanityOrCrudeHumor                       *string `json:"profanityOrCrudeHumor,omitempty"`
	SexualContentGraphicAndNudity               *string `json:"sexualContentGraphicAndNudity,omitempty"`
	SexualContentOrNudity                       *string `json:"sexualContentOrNudity,omitempty"`
	ViolenceCartoonOrFantasy                    *string `json:"violenceCartoonOrFantasy,omitempty"`
	ViolenceRealistic                           *string `json:"violenceRealistic,omitempty"`
	ViolenceRealisticProlongedGraphicOrSadistic *string `json:"violenceRealisticProlongedGraphicOrSadistic,omitempty"`

	// Yes/no facts about the app.
	Advertising              *bool `json:"advertising,omitempty"`
	AgeAssurance             *bool `json:"ageAssurance,omitempty"`
	Gambling                 *bool `json:"gambling,omitempty"`
	HealthOrWellnessTopics   *bool `json:"healthOrWellnessTopics,omitempty"`
	LootBox                  *bool `json:"lootBox,omitempty"`
	MessagingAndChat         *bool `json:"messagingAndChat,omitempty"`
	ParentalControls         *bool `json:"parentalControls,omitempty"`
	SocialMedia              *bool `json:"socialMedia,omitempty"`
	SocialMediaAgeRestricted *bool `json:"socialMediaAgeRestricted,omitempty"`
	UnrestrictedWebAccess    *bool `json:"unrestrictedWebAccess,omitempty"`
	UserGeneratedContent     *bool `json:"userGeneratedContent,omitempty"`

	// Bands and overrides.
	KidsAgeBand               *string `json:"kidsAgeBand,omitempty"`
	AgeRatingOverride         *string `json:"ageRatingOverride,omitempty"`
	AgeRatingOverrideV2       *string `json:"ageRatingOverrideV2,omitempty"`
	KoreaAgeRatingOverride    *string `json:"koreaAgeRatingOverride,omitempty"`
	DeveloperAgeRatingInfoURL *string `json:"developerAgeRatingInfoUrl,omitempty"`
}

// AgeRatingDeclarationUpdateRequest patches the declaration.
//
// There is no create request: Apple makes the record with the app.
type AgeRatingDeclarationUpdateRequest struct {
	Type       string                         `json:"type"`
	ID         string                         `json:"id"`
	Attributes AgeRatingDeclarationAttributes `json:"attributes"`
}
