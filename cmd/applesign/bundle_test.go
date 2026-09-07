package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validBundleJSON = `{
  "bundle_identifier": "com.example.myapp",
  "certificates": [
    {
      "key": "development",
      "certificate_type": "IOS_DEVELOPMENT",
      "certificate_content": "QUJD",
      "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\n",
      "expiration_date": "2027-01-01T00:00:00Z"
    }
  ],
  "profiles": [
    {
      "name": "MyApp App Store",
      "uuid": "11111111-2222-3333-4444-555555555555",
      "profile_type": "IOS_APP_STORE",
      "profile_content": "QUJD",
      "expiration_date": "2027-01-01T00:00:00Z"
    }
  ]
}`

func TestDecodeBundle(t *testing.T) {
	t.Parallel()

	bundle, err := DecodeBundle(strings.NewReader(validBundleJSON))
	if err != nil {
		t.Fatalf("DecodeBundle() error = %v", err)
	}

	if got, want := bundle.BundleIdentifier, "com.example.myapp"; got != want {
		t.Errorf("BundleIdentifier = %q, want %q", got, want)
	}
	if got, want := len(bundle.Certificates), 1; got != want {
		t.Fatalf("len(Certificates) = %d, want %d", got, want)
	}
	if got, want := bundle.Certificates[0].CertificateType, "IOS_DEVELOPMENT"; got != want {
		t.Errorf("CertificateType = %q, want %q", got, want)
	}
	if got, want := len(bundle.Profiles), 1; got != want {
		t.Fatalf("len(Profiles) = %d, want %d", got, want)
	}
	if got, want := bundle.Profiles[0].UUID, "11111111-2222-3333-4444-555555555555"; got != want {
		t.Errorf("UUID = %q, want %q", got, want)
	}
}

// A newer module must be able to add to the bundle without breaking an older
// applesign, so unknown fields are not an error.
func TestDecodeBundleIgnoresUnknownFields(t *testing.T) {
	t.Parallel()

	withExtra := strings.Replace(validBundleJSON,
		`"bundle_identifier": "com.example.myapp",`,
		`"bundle_identifier": "com.example.myapp", "something_new": {"nested": true},`, 1)

	if _, err := DecodeBundle(strings.NewReader(withExtra)); err != nil {
		t.Fatalf("DecodeBundle() error = %v", err)
	}
}

// A bundle with no profiles is the normal shape for a devices-free, CI-only
// configuration.
func TestDecodeBundleWithoutProfiles(t *testing.T) {
	t.Parallel()

	json := `{"bundle_identifier":"com.example.myapp","certificates":[{"key":"distribution",
	  "certificate_content":"QUJD","private_key_pem":"key"}],"profiles":[]}`

	bundle, err := DecodeBundle(strings.NewReader(json))
	if err != nil {
		t.Fatalf("DecodeBundle() error = %v", err)
	}
	if len(bundle.Profiles) != 0 {
		t.Errorf("len(Profiles) = %d, want 0", len(bundle.Profiles))
	}
}

func TestDecodeBundleErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		json         string
		wantContains string
	}{
		"empty":                 {json: "   \n", wantContains: "empty"},
		"terraform null output": {json: "null", wantContains: "null"},
		"not JSON":              {json: "{oh dear", wantContains: "not valid JSON"},
		"no certificates": {
			json:         `{"bundle_identifier":"com.example.myapp","certificates":[]}`,
			wantContains: "no certificates",
		},
		"certificate without content": {
			json:         `{"certificates":[{"key":"development","private_key_pem":"key"}]}`,
			wantContains: "certificate_content is empty",
		},
		"certificate without key": {
			json:         `{"certificates":[{"key":"development","certificate_content":"QUJD"}]}`,
			wantContains: "private_key_pem is empty",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeBundle(strings.NewReader(test.json))
			if err == nil {
				t.Fatal("DecodeBundle() succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.wantContains) {
				t.Errorf("DecodeBundle() error = %v, want it to contain %q", err, test.wantContains)
			}
		})
	}
}

// Paths are accepted so that process substitution works; stdin remains the
// documented route.
func TestReadBundleFromFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, []byte(validBundleJSON), 0o600); err != nil {
		t.Fatalf("writing the test bundle: %v", err)
	}

	bundle, err := readBundle(path)
	if err != nil {
		t.Fatalf("readBundle() error = %v", err)
	}
	if bundle.BundleIdentifier != "com.example.myapp" {
		t.Errorf("BundleIdentifier = %q", bundle.BundleIdentifier)
	}

	if _, err := readBundle(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("readBundle() succeeded on a missing file")
	}
}

func TestCertificateLabel(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		certificate Certificate
		want        string
	}{
		"key wins":       {Certificate{Key: "development", CertificateType: "IOS_DEVELOPMENT"}, "development"},
		"type is next":   {Certificate{CertificateType: "IOS_DISTRIBUTION"}, "IOS_DISTRIBUTION"},
		"neither is set": {Certificate{}, "certificate"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := test.certificate.label(); got != test.want {
				t.Errorf("label() = %q, want %q", got, test.want)
			}
		})
	}
}
