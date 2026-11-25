// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// csrMatchesCertificate reports whether a PEM certificate signing request
// carries the same public key as an issued certificate.
//
// certificateContent is what Apple returns: base64 DER with no PEM armour. A
// PEM certificate is accepted too. Only the public keys are compared, so a CSR
// generated later from the private key the certificate was issued for matches
// as well as the original does.
func csrMatchesCertificate(csrPEM, certificateContent string) (bool, error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		return false, errors.New("csr_content is not PEM-encoded")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("parsing csr_content: %w", err)
	}

	der, err := certificateDER(certificateContent)
	if err != nil {
		return false, err
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return false, fmt.Errorf("parsing certificate_content: %w", err)
	}

	csrKey, err := x509.MarshalPKIXPublicKey(csr.PublicKey)
	if err != nil {
		return false, fmt.Errorf("reading the CSR's public key: %w", err)
	}
	certificateKey, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return false, fmt.Errorf("reading the certificate's public key: %w", err)
	}

	return bytes.Equal(csrKey, certificateKey), nil
}

// certificateDER decodes certificate_content, base64 DER as Apple returns it,
// or a PEM certificate.
func certificateDER(content string) ([]byte, error) {
	if block, _ := pem.Decode([]byte(content)); block != nil {
		return block.Bytes, nil
	}
	compact := strings.Join(strings.Fields(content), "")
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("certificate_content is neither PEM nor base64: %w", err)
	}
	return der, nil
}

// csrRequiresReplace is the csr_content replacement rule.
//
// A changed CSR normally means a new certificate, since Apple issues against
// the CSR and never re-signs one. The exception is adoption: an imported
// certificate has no CSR in state, because Apple returns csrContent as null,
// and a configuration that then names the CSR it was issued from would
// otherwise revoke the certificate to record a value that changes nothing.
// When state holds no CSR and the configured one carries the certificate's
// public key, the change is recorded in place; Update carries the issued
// certificate forward. A CSR from a different key still replaces, with a
// warning, so the replacement is not mistaken for that adoption.
func csrRequiresReplace(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = true

	if !req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		return
	}

	var content types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("certificate_content"), &content)...)
	if resp.Diagnostics.HasError() || content.IsNull() || content.IsUnknown() || content.ValueString() == "" {
		return
	}

	matches, err := csrMatchesCertificate(req.PlanValue.ValueString(), content.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeWarning(req.Path,
			"Could Not Compare the CSR With the Certificate",
			fmt.Sprintf("The certificate has no CSR in state, as after an import, so a CSR carrying its public key "+
				"would be adopted without replacing it. The comparison failed, so the certificate will be replaced "+
				"(and the current one revoked): %s", err))
		return
	}
	if !matches {
		resp.Diagnostics.AddAttributeWarning(req.Path,
			"CSR Is From a Different Key",
			"The certificate has no CSR in state, as after an import, and the configured CSR's public key differs "+
				"from the certificate's. Adopting a certificate needs a CSR generated from the private key it was "+
				"issued for. As configured, the certificate will be revoked and a new one issued from this CSR.")
		return
	}

	resp.RequiresReplace = false
}
