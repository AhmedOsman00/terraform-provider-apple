// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

import "time"

// Certificate type enumeration.
type CertificateType string

const (
	IOSDevelopment           CertificateType = "IOS_DEVELOPMENT"
	IOSDistribution          CertificateType = "IOS_DISTRIBUTION"
	MacAppDevelopment        CertificateType = "MAC_APP_DEVELOPMENT"
	MacAppDistribution       CertificateType = "MAC_APP_DISTRIBUTION"
	MacInstallerDistribution CertificateType = "MAC_INSTALLER_DISTRIBUTION"
	DeveloperIDKext          CertificateType = "DEVELOPER_ID_KEXT"
	DeveloperIDApplication   CertificateType = "DEVELOPER_ID_APPLICATION"
	Development              CertificateType = "DEVELOPMENT"
	Distribution             CertificateType = "DISTRIBUTION"
	PassTypeID               CertificateType = "PASS_TYPE_ID"
	PassTypeIDWithNFC        CertificateType = "PASS_TYPE_ID_WITH_NFC"
	DeveloperIDInstaller     CertificateType = "DEVELOPER_ID_INSTALLER"
	DeveloperIDApplicationG2 CertificateType = "DEVELOPER_ID_APPLICATION_G2"
	DeveloperIDInstallerG2   CertificateType = "DEVELOPER_ID_INSTALLER_G2"
	DeveloperIDKextG2        CertificateType = "DEVELOPER_ID_KEXT_G2"
)

// Certificate resource model.
type Certificate struct {
	Type       string                `json:"type"`
	ID         string                `json:"id"`
	Attributes CertificateAttributes `json:"attributes"`
	Links      *ResourceLinks        `json:"links,omitempty"`
}

// Request models for Certificate operations.
type CertificateCreateRequest struct {
	Type       string                      `json:"type"`
	Attributes CertificateCreateAttributes `json:"attributes"`
}

// Certificate attributes for API responses.
type CertificateAttributes struct {
	SerialNumber       string            `json:"serialNumber"`
	CertificateContent string            `json:"certificateContent"`
	DisplayName        string            `json:"displayName"`
	Name               string            `json:"name"`
	CsrContent         string            `json:"csrContent,omitempty"`
	Platform           *BundleIDPlatform `json:"platform,omitempty"`
	ExpirationDate     *time.Time        `json:"expirationDate,omitempty"`
	CertificateType    CertificateType   `json:"certificateType"`
	RequesterFirstName string            `json:"requesterFirstName,omitempty"`
	RequesterLastName  string            `json:"requesterLastName,omitempty"`
	RequesterEmail     string            `json:"requesterEmail,omitempty"`
}

// Certificate attributes for create requests.
type CertificateCreateAttributes struct {
	CertificateType CertificateType `json:"certificateType"`
	CsrContent      string          `json:"csrContent"`
}
