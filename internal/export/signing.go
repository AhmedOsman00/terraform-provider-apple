// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/zclconf/go-cty/cty"
)

// exportSigning writes the Developer Portal: Bundle IDs and their
// capabilities, devices, merchant and pass type IDs, certificates and
// profiles. They go in one file, because a profile refers to all the rest.
func (e *exporter) exportSigning() error {
	f := e.file("signing.tf")

	e.progress("Reading Bundle IDs")
	if bundleIDs, err := e.src.GetBundleIDs(); err != nil {
		if err := e.failed("Bundle IDs", err); err != nil {
			return err
		}
	} else {
		for _, bundleID := range bundleIDs {
			if err := e.exportBundleID(f, bundleID); err != nil {
				return err
			}
		}
	}

	e.progress("Reading devices")
	if devices, err := e.src.GetDevices(); err != nil {
		if err := e.failed("devices", err); err != nil {
			return err
		}
	} else {
		for _, device := range devices {
			e.exportDevice(f, device)
		}
	}

	e.progress("Reading merchant IDs")
	if merchants, err := e.src.GetMerchantIDs(); err != nil {
		if err := e.failed("merchant IDs", err); err != nil {
			return err
		}
	} else {
		for _, merchant := range merchants {
			b, _ := e.resource(f, "apple_merchant_id", merchant.ID, merchant.Attributes.Identifier, merchant.Attributes.Identifier)
			b.str("identifier", merchant.Attributes.Identifier)
			b.str("display_name", merchant.Attributes.DisplayName)
		}
	}

	e.progress("Reading pass type IDs")
	if passTypes, err := e.src.GetPassTypeIDs(); err != nil {
		if err := e.failed("pass type IDs", err); err != nil {
			return err
		}
	} else {
		for _, passType := range passTypes {
			b, _ := e.resource(f, "apple_pass_type_id", passType.ID, passType.Attributes.Identifier, passType.Attributes.Identifier)
			b.str("identifier", passType.Attributes.Identifier)
			b.str("name", passType.Attributes.Name)
		}
	}

	e.progress("Reading certificates")
	if certificates, err := e.src.GetCertificates(); err != nil {
		if err := e.failed("certificates", err); err != nil {
			return err
		}
	} else {
		for _, certificate := range certificates {
			e.exportCertificate(f, certificate)
		}
	}

	e.progress("Reading profiles")
	profiles, err := e.src.GetProfiles()
	if err != nil {
		return e.failed("profiles", err)
	}
	for _, profile := range profiles {
		if err := e.exportProfile(f, profile); err != nil {
			return err
		}
	}

	return nil
}

func (e *exporter) exportBundleID(f *file, bundleID models.BundleID) error {
	identifier := bundleID.Attributes.Identifier
	b, addr := e.resource(f, "apple_bundle_id", bundleID.ID, identifier, identifier)
	b.str("identifier", identifier)
	b.str("name", bundleID.Attributes.Name)
	// Apple stores every Bundle ID as UNIVERSAL whatever it was created as,
	// and an import records what Apple reports.
	b.str("platform", string(bundleID.Attributes.Platform))
	if bundleID.Attributes.Platform == models.UNIVERSAL {
		e.note(Attention, addr.String(),
			"platform is UNIVERSAL because that is what Apple reports for every Bundle ID. If this is an iOS or macOS app, you can change it to IOS or MAC_OS; that change is recorded in place and does not replace the Bundle ID.")
	}

	capabilities, err := e.src.GetBundleIDCapabilities(bundleID.ID)
	if err != nil {
		return e.failed("the capabilities of "+identifier, err)
	}
	for _, capability := range capabilities {
		capabilityType := string(capability.Attributes.CapabilityType)
		c, _ := e.resource(f, "apple_bundle_id_capability", capability.ID,
			bundleID.ID+"/"+capability.ID, identifier, capabilityType)
		c.set("bundle_id", ref(addr.traversal("id")))
		c.str("capability_type", capabilityType)
		if settings := capabilitySettings(capability.Attributes.Settings); settings != nil {
			c.objects("settings", settings)
		}
	}

	return nil
}

// capabilitySettings mirrors the provider's SettingsInputFromAPI: a setting
// with one option is written as its value, one with several as its options.
func capabilitySettings(settings []models.CapabilitySetting) []map[string]cty.Value {
	if len(settings) == 0 {
		return nil
	}

	objects := make([]map[string]cty.Value, 0, len(settings))
	for _, setting := range settings {
		object := map[string]cty.Value{"key": cty.StringVal(setting.Key)}
		switch {
		case len(setting.Options) == 1:
			object["value"] = cty.StringVal(setting.Options[0].Key)
		case len(setting.Options) > 1:
			options := make([]cty.Value, 0, len(setting.Options))
			for _, option := range setting.Options {
				options = append(options, cty.ObjectVal(map[string]cty.Value{"key": cty.StringVal(option.Key)}))
			}
			object["options"] = cty.TupleVal(options)
		}
		objects = append(objects, object)
	}
	return objects
}

func (e *exporter) exportDevice(f *file, device models.Device) {
	// A disabled device cannot be deleted either, so it stays in the
	// account for good; managing it would only add noise.
	if device.Attributes.Status == models.DeviceStatusDISABLED {
		e.note(Skipped, "", "Device %q (%s) is disabled, so it was left out.", device.Attributes.Name, device.Attributes.UDID)
		return
	}

	b, _ := e.resource(f, "apple_device", device.ID, device.Attributes.UDID, device.Attributes.Name)
	b.str("name", device.Attributes.Name)
	b.str("udid", device.Attributes.UDID)
	b.str("platform", string(device.Attributes.Platform))
}

// exportCertificate writes a data source, never a resource. Importing an
// apple_certificate reissues it on the next apply -- csr_content forces
// replacement and Apple does not return it -- and reissuing revokes the
// certificate every machine is signing with.
func (e *exporter) exportCertificate(f *file, certificate models.Certificate) {
	b, addr := e.dataSource(f, "apple_certificates", certificate.ID,
		certificate.Attributes.DisplayName, string(certificate.Attributes.CertificateType))
	b.str("serial_number", certificate.Attributes.SerialNumber)

	e.note(Attention, addr.String(),
		"Certificate %q (serial %s) is read through a data source rather than imported: importing an apple_certificate reissues it, which revokes the one you are signing with. To have Terraform issue its successor, see the Code Signing guide.",
		certificate.Attributes.DisplayName, certificate.Attributes.SerialNumber)
}

func (e *exporter) exportProfile(f *file, profile models.Profile) error {
	name := profile.Attributes.Name
	if profile.Attributes.ProfileState != nil && *profile.Attributes.ProfileState != models.ProfileStateActive {
		e.note(Skipped, "", "Profile %q is %s, so it was left out.", name, *profile.Attributes.ProfileState)
		return nil
	}
	if profile.Attributes.ProfileType == nil {
		e.note(Skipped, "", "Profile %q has no type Apple reports, so it was left out.", name)
		return nil
	}

	linkages, err := e.src.GetProfileLinkages(profile.ID)
	if err != nil {
		return e.failed("the bundle ID, certificates and devices of profile "+name, err)
	}

	b, _ := e.resource(f, "apple_profile", profile.ID, profile.ID, name)
	b.str("name", name)
	b.str("profile_type", string(*profile.Attributes.ProfileType))
	b.set("bundle_id", e.idOf("apple_bundle_id", linkages.BundleID))

	certificates := make([]value, 0, len(linkages.CertificateIDs))
	for _, id := range linkages.CertificateIDs {
		if addr, ok := e.addresses["data.apple_certificates/"+id]; ok {
			certificates = append(certificates, ref(addr.traversal("certificates", 0, "id")))
		} else {
			certificates = append(certificates, literal(cty.StringVal(id)))
		}
	}
	b.refs("certificates", certificates)

	if len(linkages.DeviceIDs) > 0 {
		devices := make([]value, 0, len(linkages.DeviceIDs))
		for _, id := range linkages.DeviceIDs {
			devices = append(devices, e.idOf("apple_device", id))
		}
		b.refs("devices", devices)
	}

	return nil
}
