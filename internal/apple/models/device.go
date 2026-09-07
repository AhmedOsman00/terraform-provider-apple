// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

import "time"

// Device platform enumeration.
type DevicePlatform string

const (
	DeviceIOS    DevicePlatform = "IOS"
	DeviceMAC    DevicePlatform = "MAC_OS"
	DeviceTVOS   DevicePlatform = "TV_OS"
	DeviceVision DevicePlatform = "VISION_OS"
)

// Device class enumeration.
type DeviceClass string

const (
	DeviceClassIPHONE           DeviceClass = "IPHONE"
	DeviceClassIPAD             DeviceClass = "IPAD"
	DeviceClassIPOD             DeviceClass = "IPOD"
	DeviceClassAPPLE_TV         DeviceClass = "APPLE_TV"
	DeviceClassAPPLE_WATCH      DeviceClass = "APPLE_WATCH"
	DeviceClassMAC              DeviceClass = "MAC"
	DeviceClassAPPLE_VISION_PRO DeviceClass = "APPLE_VISION_PRO"
)

// Device status enumeration.
type DeviceStatus string

const (
	DeviceStatusENABLED    DeviceStatus = "ENABLED"
	DeviceStatusDISABLED   DeviceStatus = "DISABLED"
	DeviceStatusPROCESSING DeviceStatus = "PROCESSING"
	DeviceStatusINELIGIBLE DeviceStatus = "INELIGIBLE"
)

// Device resource model.
type Device struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Attributes DeviceAttributes `json:"attributes"`
	Links      *ResourceLinks   `json:"links,omitempty"`
}

// Request models for Device operations.
type DeviceCreateRequest struct {
	Type       string                 `json:"type"`
	Attributes DeviceCreateAttributes `json:"attributes"`
}

type DeviceUpdateRequest struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Attributes DeviceUpdateAttributes `json:"attributes"`
}

// Device attributes.
type DeviceAttributes struct {
	AddedDate   *time.Time     `json:"addedDate,omitempty"`
	Name        string         `json:"name"`
	DeviceClass DeviceClass    `json:"deviceClass"`
	Model       *string        `json:"model,omitempty"`
	UDID        string         `json:"udid"`
	Platform    DevicePlatform `json:"platform"`
	Status      DeviceStatus   `json:"status"`
}

// Create attributes for Device creation.
type DeviceCreateAttributes struct {
	Name     string         `json:"name"`
	UDID     string         `json:"udid"`
	Platform DevicePlatform `json:"platform"`
}

// Update attributes (only name can be updated for Devices).
type DeviceUpdateAttributes struct {
	Name   string        `json:"name"`
	Status *DeviceStatus `json:"status,omitempty"`
}
