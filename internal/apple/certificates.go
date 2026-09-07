package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-apple/internal/apple/models"
)

// GetCertificates retrieves all Certificates for the team.
func (c *Client) GetCertificates() ([]models.Certificate, error) {
	return getAllPages[models.Certificate](c, "/v1/certificates", defaultPageSize)
}

// GetCertificate retrieves a specific Certificate by its ID.
func (c *Client) GetCertificate(certificateID string) (*models.Certificate, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/certificates/%s", c.HostURL, certificateID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Certificate]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateCertificate creates a new Certificate.
func (c *Client) CreateCertificate(certificateType models.CertificateType, csrContent string, authToken *string) (*models.Certificate, error) {
	certificateRequest := models.CertificateCreateRequest{
		Type: "certificates",
		Attributes: models.CertificateCreateAttributes{
			CertificateType: certificateType,
			CsrContent:      csrContent,
		},
	}

	requestData := models.Request[models.CertificateCreateRequest]{
		Data: certificateRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/certificates", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Certificate]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteCertificate revokes a Certificate.
func (c *Client) DeleteCertificate(certificateID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/certificates/%s", c.HostURL, certificateID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	if err != nil {
		return err
	}

	// DELETE requests return 204 No Content on success, which is handled by doRequest
	return nil
}

// GetCertificateBySerialNumber finds a Certificate by its serial number.
func (c *Client) GetCertificateBySerialNumber(serialNumber string) (*models.Certificate, error) {
	certificates, err := c.GetCertificates()
	if err != nil {
		return nil, err
	}

	for _, certificate := range certificates {
		if certificate.Attributes.SerialNumber == serialNumber {
			return &certificate, nil
		}
	}

	return nil, fmt.Errorf("certificate with serial number '%s' not found", serialNumber)
}

// GetCertificateByName finds a Certificate by its display name.
func (c *Client) GetCertificateByName(name string) (*models.Certificate, error) {
	certificates, err := c.GetCertificates()
	if err != nil {
		return nil, err
	}

	for _, certificate := range certificates {
		if certificate.Attributes.DisplayName == name || certificate.Attributes.Name == name {
			return &certificate, nil
		}
	}

	return nil, fmt.Errorf("certificate with name '%s' not found", name)
}
