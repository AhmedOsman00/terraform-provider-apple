// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func testCSR(t *testing.T, key crypto.Signer, commonName string) string {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: commonName},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

// testCertificate issues a certificate for key, returned the way Apple returns
// certificateContent: base64 DER with no PEM armour.
func testCertificate(t *testing.T, key crypto.Signer) string {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "iPhone Distribution: Example"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestCSRMatchesCertificate(t *testing.T) {
	key := testRSAKey(t)
	other := testRSAKey(t)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := testCertificate(t, key)
	der, _ := base64.StdEncoding.DecodeString(certificate)
	certificatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	tests := []struct {
		name        string
		csr         string
		certificate string
		want        bool
		wantErr     string
	}{
		{name: "the CSR it was issued from", csr: testCSR(t, key, "original"), certificate: certificate, want: true},
		{name: "a CSR generated later from the same key", csr: testCSR(t, key, "regenerated"), certificate: certificate, want: true},
		{name: "a PEM certificate", csr: testCSR(t, key, "original"), certificate: certificatePEM, want: true},
		{name: "base64 wrapped across lines", csr: testCSR(t, key, "original"), certificate: certificate[:64] + "\n" + certificate[64:], want: true},
		{name: "a CSR from another key", csr: testCSR(t, other, "other"), certificate: certificate, want: false},
		{name: "a CSR from another key type", csr: testCSR(t, ecKey, "ec"), certificate: certificate, want: false},
		{name: "not a CSR", csr: "not a csr", certificate: certificate, wantErr: "not PEM-encoded"},
		{name: "not a certificate", csr: testCSR(t, key, "original"), certificate: "%%%", wantErr: "neither PEM nor base64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := csrMatchesCertificate(tt.csr, tt.certificate)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("csrMatchesCertificate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSRRequiresReplace(t *testing.T) {
	ctx := context.Background()
	key := testRSAKey(t)
	certificate := testCertificate(t, key)
	sameKey := testCSR(t, key, "regenerated")
	otherKey := testCSR(t, testRSAKey(t), "other")

	var schemaResp resource.SchemaResponse
	(&certificateResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	tests := []struct {
		name        string
		stateCSR    types.String
		content     types.String
		planCSR     types.String
		wantReplace bool
		wantWarning string
	}{
		{name: "adopting the CSR of an imported certificate", stateCSR: types.StringNull(), content: types.StringValue(certificate), planCSR: types.StringValue(sameKey), wantReplace: false},
		{name: "a CSR from another key on an imported certificate", stateCSR: types.StringNull(), content: types.StringValue(certificate), planCSR: types.StringValue(otherKey), wantReplace: true, wantWarning: "CSR Is From a Different Key"},
		{name: "changing a CSR state already holds", stateCSR: types.StringValue(otherKey), content: types.StringValue(certificate), planCSR: types.StringValue(sameKey), wantReplace: true},
		{name: "a CSR not known until apply", stateCSR: types.StringNull(), content: types.StringValue(certificate), planCSR: types.StringUnknown(), wantReplace: true},
		{name: "no certificate in state to compare with", stateCSR: types.StringNull(), content: types.StringNull(), planCSR: types.StringValue(sameKey), wantReplace: true},
		{name: "a certificate that does not parse", stateCSR: types.StringNull(), content: types.StringValue("%%%"), planCSR: types.StringValue(sameKey), wantReplace: true, wantWarning: "Could Not Compare"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tfsdk.State{Schema: schemaResp.Schema}
			diags := state.Set(ctx, &certificateResourceModel{
				ID:                 types.StringValue("CERT1"),
				CertificateType:    types.StringValue("IOS_DISTRIBUTION"),
				CsrContent:         tt.stateCSR,
				SerialNumber:       types.StringValue("1"),
				CertificateContent: tt.content,
				DisplayName:        types.StringNull(),
				Name:               types.StringNull(),
				Platform:           types.StringNull(),
				ExpirationDate:     types.StringNull(),
				RequesterFirstName: types.StringNull(),
				RequesterLastName:  types.StringNull(),
				RequesterEmail:     types.StringNull(),
				EarlyRenewalHours:  types.Int64Null(),
				ReadyForRenewal:    types.BoolValue(false),
			})
			if diags.HasError() {
				t.Fatalf("building state: %v", diags)
			}

			req := planmodifier.StringRequest{
				Path:       path.Root("csr_content"),
				State:      state,
				StateValue: tt.stateCSR,
				PlanValue:  tt.planCSR,
			}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			csrRequiresReplace(ctx, req, resp)

			if resp.RequiresReplace != tt.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tt.wantReplace)
			}
			if resp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", resp.Diagnostics)
			}
			warnings := resp.Diagnostics.Warnings()
			switch {
			case tt.wantWarning == "" && len(warnings) > 0:
				t.Errorf("unexpected warning: %s", warnings[0].Summary())
			case tt.wantWarning != "" && (len(warnings) != 1 || !strings.Contains(warnings[0].Summary(), tt.wantWarning)):
				t.Errorf("warnings = %v, want one containing %q", warnings, tt.wantWarning)
			}
		})
	}
}
