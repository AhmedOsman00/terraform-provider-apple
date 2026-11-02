// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetCiWorkflows lists active workflows. A deleted workflow can still be read by ID.
func (c *Client) GetCiWorkflows(productID string) ([]models.CiWorkflow, error) {
	return getAllPages[models.CiWorkflow](c, "/v1/ciProducts/"+url.PathEscape(productID)+"/workflows", defaultPageSize)
}

func (c *Client) GetCiProducts() ([]models.CiProduct, error) {
	return getAllPages[models.CiProduct](c, "/v1/ciProducts", defaultPageSize)
}
func (c *Client) GetScmRepositories() ([]models.ScmRepository, error) {
	return getAllPages[models.ScmRepository](c, "/v1/scmRepositories", defaultPageSize)
}
func (c *Client) GetCiXcodeVersions() ([]models.CiXcodeVersion, error) {
	return getAllPages[models.CiXcodeVersion](c, "/v1/ciXcodeVersions", defaultPageSize)
}
func (c *Client) GetCiMacOsVersions() ([]models.CiMacOsVersion, error) {
	return getAllPages[models.CiMacOsVersion](c, "/v1/ciMacOsVersions", defaultPageSize)
}
func (c *Client) GetCiWorkflow(id string) (*models.CiWorkflow, error) {
	return c.ciWorkflowRequest(http.MethodGet, "/v1/ciWorkflows/"+url.PathEscape(id)+"?include=product,repository,xcodeVersion,macOsVersion", nil)
}
func (c *Client) CreateCiWorkflow(workflow models.CiWorkflow) (*models.CiWorkflow, error) {
	workflow.Type = "ciWorkflows"
	workflow.ID = ""
	return c.ciWorkflowRequest(http.MethodPost, "/v1/ciWorkflows", &workflow)
}
func (c *Client) UpdateCiWorkflow(workflow models.CiWorkflow) (*models.CiWorkflow, error) {
	workflow.Type = "ciWorkflows"
	// Product and repository are immutable and absent from Apple's update request.
	if workflow.Relationships != nil {
		workflow.Relationships = &models.CiWorkflowRelationships{XcodeVersion: workflow.Relationships.XcodeVersion, MacOsVersion: workflow.Relationships.MacOsVersion}
	}
	return c.ciWorkflowRequest(http.MethodPatch, "/v1/ciWorkflows/"+url.PathEscape(workflow.ID), &workflow)
}
func (c *Client) DeleteCiWorkflow(id string) error {
	req, err := http.NewRequest(http.MethodDelete, c.HostURL+"/v1/ciWorkflows/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	_, err = c.doRequest(req, nil)
	return err
}
func (c *Client) ciWorkflowRequest(method, path string, workflow *models.CiWorkflow) (*models.CiWorkflow, error) {
	var body []byte
	if workflow != nil {
		var err error
		body, err = json.Marshal(models.Request[models.CiWorkflow]{Data: *workflow})
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, c.HostURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	result, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}
	var response models.Response[models.CiWorkflow]
	if err = json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("decode CI workflow: %w", err)
	}
	return &response.Data, nil
}
