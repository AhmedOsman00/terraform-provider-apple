// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

import "encoding/json"

// Bundle ID Capability resource model.
type BundleIDCapability struct {
	Type       string                       `json:"type"`
	ID         string                       `json:"id"`
	Attributes BundleIDCapabilityAttributes `json:"attributes"`
	Links      *ResourceLinks               `json:"links,omitempty"`
}

// Bundle ID Capability attributes.
type BundleIDCapabilityAttributes struct {
	CapabilityType CapabilityType      `json:"capabilityType"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// Request models for Bundle ID Capability operations.
//
// The bundleId relationship is part of the primary resource object, not a
// sibling of it: JSON:API nests relationships inside "data", and Apple rejects
// a top-level "relationships" key with a 422 "Unexpected or invalid value at
// 'relationships'".
type BundleIDCapabilityCreateRequest struct {
	Type          string                                `json:"type"`
	Attributes    BundleIDCapabilityAttributes          `json:"attributes"`
	Relationships BundleIDCapabilityCreateRelationships `json:"relationships"`
}

// Bundle ID Capability relationships for creation.
type BundleIDCapabilityCreateRelationships struct {
	BundleID ResourceIdentifier `json:"bundleId"`
}

type BundleIDCapabilityUpdateRequest struct {
	Type       string                             `json:"type"`
	ID         string                             `json:"id"`
	Attributes BundleIDCapabilityUpdateAttributes `json:"attributes"`
}

// Update attributes for Bundle ID Capabilities.
type BundleIDCapabilityUpdateAttributes struct {
	CapabilityType *CapabilityType     `json:"capabilityType,omitempty"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// Capability types enumeration.
//
// This is exactly the set Apple accepts on POST /v1/bundleIdCapabilities, taken
// from the 409 it answers when given anything else. Several names differ from
// the Apple framework spellings a reader would expect -- HEALTHKIT, HOMEKIT,
// CLASSKIT and SIRIKIT have no underscore, and Wallet is WALLET rather than
// WALLET_PASSES -- so do not "correct" them. Types the App Store Connect API
// does not model at all (APP_ATTEST, WEATHER_KIT, GROUP_ACTIVITIES and the
// rest) are deliberately absent: they are configured in Xcode, not here.
//
// InAppPurchaseCapability carries the suffix because the bare name belongs to
// the App Store Connect product in in_app_purchase.go -- the Bundle ID
// capability and the purchase itself are unrelated resources.
type CapabilityType string

const (
	AccessWifiInformation          CapabilityType = "ACCESS_WIFI_INFORMATION"
	AppGroups                      CapabilityType = "APP_GROUPS"
	ApplePay                       CapabilityType = "APPLE_PAY"
	AppleIDAuth                    CapabilityType = "APPLE_ID_AUTH"
	AssociatedDomains              CapabilityType = "ASSOCIATED_DOMAINS"
	AutofillCredentialProvider     CapabilityType = "AUTOFILL_CREDENTIAL_PROVIDER"
	ClassKit                       CapabilityType = "CLASSKIT"
	CoreMediaHLSLowLatency         CapabilityType = "COREMEDIA_HLS_LOW_LATENCY"
	DataProtection                 CapabilityType = "DATA_PROTECTION"
	GameCenter                     CapabilityType = "GAME_CENTER"
	HealthKit                      CapabilityType = "HEALTHKIT"
	HomeKit                        CapabilityType = "HOMEKIT"
	HotSpot                        CapabilityType = "HOT_SPOT"
	ICloud                         CapabilityType = "ICLOUD"
	InAppPurchaseCapability        CapabilityType = "IN_APP_PURCHASE"
	InterAppAudio                  CapabilityType = "INTER_APP_AUDIO"
	Maps                           CapabilityType = "MAPS"
	Multipath                      CapabilityType = "MULTIPATH"
	NetworkCustomProtocol          CapabilityType = "NETWORK_CUSTOM_PROTOCOL"
	NetworkExtensions              CapabilityType = "NETWORK_EXTENSIONS"
	NfcTagReading                  CapabilityType = "NFC_TAG_READING"
	PersonalVpn                    CapabilityType = "PERSONAL_VPN"
	PushNotifications              CapabilityType = "PUSH_NOTIFICATIONS"
	SiriKit                        CapabilityType = "SIRIKIT"
	SystemExtensionInstall         CapabilityType = "SYSTEM_EXTENSION_INSTALL"
	UserManagement                 CapabilityType = "USER_MANAGEMENT"
	Wallet                         CapabilityType = "WALLET"
	WirelessAccessoryConfiguration CapabilityType = "WIRELESS_ACCESSORY_CONFIGURATION"
)

// Capability setting for configurable capabilities.
// A capability setting carries its chosen value in options, not in a scalar
// field: Apple rejects a "value" property with a 409 "The attribute
// 'settings/0' contains additional unknown property 'value'". The provider's
// friendlier settings.value is mapped onto options[].key by SettingsToAPI.
type CapabilitySetting struct {
	Key      string                    `json:"key"`
	Name     string                    `json:"name,omitempty"`
	Visible  *bool                     `json:"visible,omitempty"`
	MinCount *int                      `json:"minCount,omitempty"`
	Options  []CapabilitySettingOption `json:"options,omitempty"`
}

// MarshalJSON sends only the two properties Apple accepts as input.
//
// Every other field on a setting is read-only: name is rejected outright with a
// 409 "Unrecognized field 'name'", and visible and minCount describe what the
// portal shows rather than what to configure. They still have to be decodable,
// because Apple returns them -- and because the provider surfaces them as
// computed attributes, they would otherwise be echoed straight back on the next
// update and fail a request that had worked on create.
func (s CapabilitySetting) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key     string                    `json:"key"`
		Options []CapabilitySettingOption `json:"options,omitempty"`
	}{
		Key:     s.Key,
		Options: s.Options,
	})
}

// Options for capability settings.
type CapabilitySettingOption struct {
	Key         string `json:"key"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// MarshalJSON sends only the option key, which is the whole of Apple's input
// form: name, description and enabled are labels it returns for display.
func (o CapabilitySettingOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key string `json:"key"`
	}{Key: o.Key})
}

// Valid capability types list for validation.
var ValidCapabilityTypes = []string{
	string(AccessWifiInformation),
	string(AppGroups),
	string(ApplePay),
	string(AppleIDAuth),
	string(AssociatedDomains),
	string(AutofillCredentialProvider),
	string(ClassKit),
	string(CoreMediaHLSLowLatency),
	string(DataProtection),
	string(GameCenter),
	string(HealthKit),
	string(HomeKit),
	string(HotSpot),
	string(ICloud),
	string(InAppPurchaseCapability),
	string(InterAppAudio),
	string(Maps),
	string(Multipath),
	string(NetworkCustomProtocol),
	string(NetworkExtensions),
	string(NfcTagReading),
	string(PersonalVpn),
	string(PushNotifications),
	string(SiriKit),
	string(SystemExtensionInstall),
	string(UserManagement),
	string(Wallet),
	string(WirelessAccessoryConfiguration),
}
