// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

type CiPattern struct {
	Pattern  *string `json:"pattern,omitempty"`
	IsPrefix *bool   `json:"isPrefix,omitempty"`
}
type CiPatterns struct {
	IsAllMatch *bool       `json:"isAllMatch,omitempty"`
	Patterns   []CiPattern `json:"patterns,omitempty"`
}
type CiFileMatcher struct {
	Directory     *string `json:"directory,omitempty"`
	FileExtension *string `json:"fileExtension,omitempty"`
	FileName      *string `json:"fileName,omitempty"`
}
type CiFilesAndFoldersRule struct {
	Mode     *string         `json:"mode,omitempty"`
	Matchers []CiFileMatcher `json:"matchers,omitempty"`
}
type CiStartCondition struct {
	Source              *CiPatterns            `json:"source,omitempty"`
	Destination         *CiPatterns            `json:"destination,omitempty"`
	FilesAndFoldersRule *CiFilesAndFoldersRule `json:"filesAndFoldersRule,omitempty"`
	AutoCancel          *bool                  `json:"autoCancel,omitempty"`
}
type CiManualStartCondition struct {
	Source      *CiPatterns `json:"source,omitempty"`
	Destination *CiPatterns `json:"destination,omitempty"`
}
type CiSchedule struct {
	Frequency *string  `json:"frequency,omitempty"`
	Days      []string `json:"days,omitempty"`
	Hour      *int64   `json:"hour,omitempty"`
	Minute    *int64   `json:"minute,omitempty"`
	Timezone  *string  `json:"timezone,omitempty"`
}
type CiScheduledStartCondition struct {
	Source   *CiPatterns `json:"source,omitempty"`
	Schedule *CiSchedule `json:"schedule,omitempty"`
}
type CiTestDestination struct {
	DeviceTypeName       *string `json:"deviceTypeName,omitempty"`
	DeviceTypeIdentifier *string `json:"deviceTypeIdentifier,omitempty"`
	RuntimeName          *string `json:"runtimeName,omitempty"`
	RuntimeIdentifier    *string `json:"runtimeIdentifier,omitempty"`
	Kind                 *string `json:"kind,omitempty"`
}
type CiTestConfiguration struct {
	Kind             *string             `json:"kind,omitempty"`
	TestPlanName     *string             `json:"testPlanName,omitempty"`
	TestDestinations []CiTestDestination `json:"testDestinations,omitempty"`
}
type CiAction struct {
	Name                      *string              `json:"name,omitempty"`
	ActionType                *string              `json:"actionType,omitempty"`
	Destination               *string              `json:"destination,omitempty"`
	BuildDistributionAudience *string              `json:"buildDistributionAudience,omitempty"`
	TestConfiguration         *CiTestConfiguration `json:"testConfiguration,omitempty"`
	Scheme                    *string              `json:"scheme,omitempty"`
	Platform                  *string              `json:"platform,omitempty"`
	IsRequiredToPass          *bool                `json:"isRequiredToPass,omitempty"`
}
type CiWorkflowAttributes struct {
	Name                            *string                    `json:"name"`
	Description                     *string                    `json:"description"`
	BranchStartCondition            *CiStartCondition          `json:"branchStartCondition"`
	TagStartCondition               *CiStartCondition          `json:"tagStartCondition"`
	PullRequestStartCondition       *CiStartCondition          `json:"pullRequestStartCondition"`
	ScheduledStartCondition         *CiScheduledStartCondition `json:"scheduledStartCondition"`
	ManualBranchStartCondition      *CiManualStartCondition    `json:"manualBranchStartCondition"`
	ManualTagStartCondition         *CiManualStartCondition    `json:"manualTagStartCondition"`
	ManualPullRequestStartCondition *CiManualStartCondition    `json:"manualPullRequestStartCondition"`
	Actions                         []CiAction                 `json:"actions"`
	IsEnabled                       *bool                      `json:"isEnabled"`
	Clean                           *bool                      `json:"clean"`
	ContainerFilePath               *string                    `json:"containerFilePath"`
}
type CiWorkflowRelationships struct {
	Product      *ResourceIdentifier `json:"product,omitempty"`
	Repository   *ResourceIdentifier `json:"repository,omitempty"`
	XcodeVersion *ResourceIdentifier `json:"xcodeVersion,omitempty"`
	MacOsVersion *ResourceIdentifier `json:"macOsVersion,omitempty"`
}
type CiWorkflow struct {
	ID            string                   `json:"id,omitempty"`
	Type          string                   `json:"type"`
	Attributes    *CiWorkflowAttributes    `json:"attributes"`
	Relationships *CiWorkflowRelationships `json:"relationships"`
}
type CiProductAttributes struct {
	Name        *string `json:"name"`
	CreatedDate *string `json:"createdDate"`
	ProductType *string `json:"productType"`
}
type CiProduct struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Attributes *CiProductAttributes `json:"attributes"`
}
type ScmRepositoryAttributes struct {
	LastAccessedDate *string `json:"lastAccessedDate"`
	HttpCloneUrl     *string `json:"httpCloneUrl"`
	SshCloneUrl      *string `json:"sshCloneUrl"`
	OwnerName        *string `json:"ownerName"`
	RepositoryName   *string `json:"repositoryName"`
}
type ScmRepository struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type"`
	Attributes *ScmRepositoryAttributes `json:"attributes"`
}
type CiXcodeVersionRuntime struct {
	RuntimeName       *string `json:"runtimeName"`
	RuntimeIdentifier *string `json:"runtimeIdentifier"`
}
type CiXcodeVersionTestDestination struct {
	DeviceTypeName       *string                 `json:"deviceTypeName"`
	DeviceTypeIdentifier *string                 `json:"deviceTypeIdentifier"`
	AvailableRuntimes    []CiXcodeVersionRuntime `json:"availableRuntimes"`
	Kind                 *string                 `json:"kind"`
}
type CiXcodeVersionAttributes struct {
	Version          *string                         `json:"version"`
	Name             *string                         `json:"name"`
	TestDestinations []CiXcodeVersionTestDestination `json:"testDestinations"`
}
type CiXcodeVersion struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes *CiXcodeVersionAttributes `json:"attributes"`
}
type CiMacOsVersionAttributes struct {
	Version *string `json:"version"`
	Name    *string `json:"name"`
}
type CiMacOsVersion struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes *CiMacOsVersionAttributes `json:"attributes"`
}
