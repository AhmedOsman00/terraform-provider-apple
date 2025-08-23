package models

// Bundle ID Capability resource model
type BundleIDCapability struct {
	Type       string                       `json:"type"`
	ID         string                       `json:"id"`
	Attributes BundleIDCapabilityAttributes `json:"attributes"`
	Links      *ResourceLinks               `json:"links,omitempty"`
}

// Bundle ID Capability attributes
type BundleIDCapabilityAttributes struct {
	CapabilityType CapabilityType      `json:"capabilityType"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// Request models for Bundle ID Capability operations
type BundleIDCapabilityCreateRequest struct {
	Type       string                       `json:"type"`
	Attributes BundleIDCapabilityAttributes `json:"attributes"`
}

type BundleIDCapabilityUpdateRequest struct {
	Type       string                             `json:"type"`
	ID         string                             `json:"id"`
	Attributes BundleIDCapabilityUpdateAttributes `json:"attributes"`
}

// Update attributes for Bundle ID Capabilities
type BundleIDCapabilityUpdateAttributes struct {
	CapabilityType *CapabilityType     `json:"capabilityType,omitempty"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// Capability types enumeration
type CapabilityType string

const (
	// Core capabilities
	AccessWifiInformation          CapabilityType = "ACCESS_WIFI_INFORMATION"
	AppAttest                      CapabilityType = "APP_ATTEST"
	AppGroups                      CapabilityType = "APP_GROUPS"
	ApplePay                       CapabilityType = "APPLE_PAY"
	ApplePayLater                  CapabilityType = "APPLE_PAY_LATER"
	AssociatedDomains              CapabilityType = "ASSOCIATED_DOMAINS"
	AutofillCredentialProvider     CapabilityType = "AUTOFILL_CREDENTIAL_PROVIDER"
	ClassKit                       CapabilityType = "CLASS_KIT"
	ICloud                         CapabilityType = "ICLOUD"
	InAppPurchase                  CapabilityType = "IN_APP_PURCHASE"
	InterAppAudio                  CapabilityType = "INTER_APP_AUDIO"
	GameCenter                     CapabilityType = "GAME_CENTER"
	HealthKit                      CapabilityType = "HEALTH_KIT"
	HomeKit                        CapabilityType = "HOME_KIT"
	HotSpot                        CapabilityType = "HOT_SPOT"
	Multipath                      CapabilityType = "MULTIPATH"
	NetworkExtensions              CapabilityType = "NETWORK_EXTENSIONS"
	NfcTagReading                  CapabilityType = "NFC_TAG_READING"
	PersonalVpn                    CapabilityType = "PERSONAL_VPN"
	PushNotifications              CapabilityType = "PUSH_NOTIFICATIONS"
	Siri                           CapabilityType = "SIRI"
	SystemExtensionInstall         CapabilityType = "SYSTEM_EXTENSION_INSTALL"
	UserManagement                 CapabilityType = "USER_MANAGEMENT"
	WalletPasses                   CapabilityType = "WALLET_PASSES"
	WirelessAccessoryConfiguration CapabilityType = "WIRELESS_ACCESSORY_CONFIGURATION"

	// Extended capabilities
	CommunicationNotifications  CapabilityType = "COMMUNICATION_NOTIFICATIONS"
	DataProtection              CapabilityType = "DATA_PROTECTION"
	ExtendedVirtualAddressing   CapabilityType = "EXTENDED_VIRTUAL_ADDRESSING"
	FileProviderTesting         CapabilityType = "FILE_PROVIDER_TESTING"
	FontsInstallation           CapabilityType = "FONTS_INSTALLATION"
	GroupActivities             CapabilityType = "GROUP_ACTIVITIES"
	MapsService                 CapabilityType = "MAPS_SERVICE"
	MdmManagedAssociatedDomains CapabilityType = "MDM_MANAGED_ASSOCIATED_DOMAINS"
	ShazKit                     CapabilityType = "SHAZKIT"
	WeatherKit                  CapabilityType = "WEATHER_KIT"
)

// Capability setting for configurable capabilities
type CapabilitySetting struct {
	Key      string                    `json:"key"`
	Name     string                    `json:"name,omitempty"`
	Value    interface{}               `json:"value"`
	Visible  *bool                     `json:"visible,omitempty"`
	MinCount *int                      `json:"minCount,omitempty"`
	Options  []CapabilitySettingOption `json:"options,omitempty"`
}

// Options for capability settings
type CapabilitySettingOption struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// Valid capability types list for validation
var ValidCapabilityTypes = []string{
	string(AccessWifiInformation),
	string(AppAttest),
	string(AppGroups),
	string(ApplePay),
	string(ApplePayLater),
	string(AssociatedDomains),
	string(AutofillCredentialProvider),
	string(ClassKit),
	string(ICloud),
	string(InAppPurchase),
	string(InterAppAudio),
	string(GameCenter),
	string(HealthKit),
	string(HomeKit),
	string(HotSpot),
	string(Multipath),
	string(NetworkExtensions),
	string(NfcTagReading),
	string(PersonalVpn),
	string(PushNotifications),
	string(Siri),
	string(SystemExtensionInstall),
	string(UserManagement),
	string(WalletPasses),
	string(WirelessAccessoryConfiguration),
	string(CommunicationNotifications),
	string(DataProtection),
	string(ExtendedVirtualAddressing),
	string(FileProviderTesting),
	string(FontsInstallation),
	string(GroupActivities),
	string(MapsService),
	string(MdmManagedAssociatedDomains),
	string(ShazKit),
	string(WeatherKit),
}
