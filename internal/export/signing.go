// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"regexp"
	"slices"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/zclconf/go-cty/cty"
)

// exportSigning writes the Developer Portal for the whole account: Bundle IDs
// and their capabilities, devices, merchant and pass type IDs, certificates
// and profiles. They go in one file, because a profile refers to all the rest.
func (e *exporter) exportSigning() error {
	f := e.file("signing.tf")

	if e.wants(PartBundleIDs) {
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
	}

	if e.wants(PartDevices) {
		if devices, ok, err := e.readDevices(); err != nil || !ok {
			return err
		} else {
			for _, device := range devices {
				e.exportDevice(f, device)
			}
		}
	}

	if err := e.exportMerchantAndPassTypeIDs(f); err != nil {
		return err
	}

	if e.wants(PartCertificates) {
		if certificates, ok, err := e.readCertificates(); err != nil || !ok {
			return err
		} else {
			for _, certificate := range certificates {
				e.exportCertificate(f, certificate)
			}
		}
	}

	if !e.wants(PartProfiles) {
		return nil
	}
	e.progress("Reading profiles")
	profiles, err := e.src.GetProfiles()
	if err != nil {
		return e.failed("profiles", err)
	}
	for _, profile := range profiles {
		if !e.exportableProfile(profile, true) {
			continue
		}
		linkages, err := e.src.GetProfileLinkages(profile.ID)
		if err != nil {
			if err := e.failed("the bundle ID, certificates and devices of profile "+profile.Attributes.Name, err); err != nil {
				return err
			}
			continue
		}
		e.exportProfile(f, profile, linkages)
	}

	return nil
}

func (e *exporter) readDevices() ([]models.Device, bool, error) {
	e.progress("Reading devices")
	devices, err := e.src.GetDevices()
	if err != nil {
		return nil, false, e.failed("devices", err)
	}
	return devices, true, nil
}

func (e *exporter) readCertificates() ([]models.Certificate, bool, error) {
	e.progress("Reading certificates")
	certificates, err := e.src.GetCertificates()
	if err != nil {
		return nil, false, e.failed("certificates", err)
	}
	return certificates, true, nil
}

// exportMerchantAndPassTypeIDs writes the two account-wide identifiers no app
// owns: a merchant or pass type ID is shared by whichever apps name it in an
// entitlement, and nothing Apple returns says which.
func (e *exporter) exportMerchantAndPassTypeIDs(f *file) error {
	if e.wants(PartMerchantIDs) {
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
	}

	if e.wants(PartPassTypeIDs) {
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
	}
	return nil
}

// appProfile is a profile issued for one of the apps being exported, with
// what it was issued for.
type appProfile struct {
	app      models.App
	profile  models.Profile
	linkages *apple.ProfileLinkages
}

// exportScopedSigning writes the signing material of the apps --app names:
// the Bundle IDs each one owns (claimBundleIDs), the profiles issued for
// those, and the certificates and devices the profiles use. Each app's goes in
// <bundle_id>_signing.tf; a certificate or device two of them use, and the
// account-wide parts --only names, go in signing.tf, so each is written once.
func (e *exporter) exportScopedSigning(selected []models.App) error {
	e.progress("Reading apps")
	all, err := e.src.GetApps()
	if err != nil {
		if err := e.failed("the team's apps, so Bundle IDs are matched against the selected apps alone", err); err != nil {
			return err
		}
		all = selected
	}

	e.progress("Reading Bundle IDs")
	bundleIDs, err := e.src.GetBundleIDs()
	if err != nil {
		if err := e.failed("Bundle IDs", err); err != nil {
			return err
		}
	}
	owners := claimBundleIDs(all, bundleIDs)

	appByID := map[string]models.App{}
	for _, app := range selected {
		appByID[app.ID] = app
	}
	ownerOf := func(bundleRecordID string) (models.App, bool) {
		app, ok := appByID[owners[bundleRecordID]]
		return app, ok
	}
	signingFile := func(app models.App) *file {
		return e.appFile(appBase(app)+"_signing.tf", app.Attributes.BundleID)
	}

	if e.wants(PartBundleIDs) {
		for _, app := range selected {
			for _, bundleID := range bundleIDs {
				if owner, ok := ownerOf(bundleID.ID); !ok || owner.ID != app.ID {
					continue
				}
				if err := e.exportBundleID(signingFile(app), bundleID); err != nil {
					return err
				}
			}
		}
	}
	unclaimed := unclaimedBundleIDs(selected, bundleIDs, owners)
	for _, app := range selected {
		for _, bundleID := range unclaimed[app.ID] {
			e.note(Skipped, "",
				"Bundle ID %s looks as though it belongs to %s, but it is outside %s.*, so it was not exported with it. If it does belong to the app, export it with --only bundle-ids and no --app.",
				bundleID.Attributes.Identifier, app.Attributes.BundleID, app.Attributes.BundleID)
		}
	}

	// The profiles decide which certificates and devices are referenced, so
	// they are read whenever any of the three is wanted.
	var profiles []appProfile
	if e.wantsAny(PartProfiles, PartCertificates, PartDevices) {
		e.progress("Reading profiles")
		all, err := e.src.GetProfiles()
		if err != nil {
			if err := e.failed("profiles", err); err != nil {
				return err
			}
		}
		for _, profile := range all {
			if !e.exportableProfile(profile, false) {
				continue
			}
			linkages, err := e.src.GetProfileLinkages(profile.ID)
			if err != nil {
				if err := e.failed("the bundle ID, certificates and devices of profile "+profile.Attributes.Name, err); err != nil {
					return err
				}
				continue
			}
			if app, ok := ownerOf(linkages.BundleID); ok {
				profiles = append(profiles, appProfile{app: app, profile: profile, linkages: linkages})
			}
		}
	}

	// usedBy maps a certificate or device ID to the apps whose profiles use
	// it, in the order they were selected.
	usedBy := func(ids func(*apple.ProfileLinkages) []string) map[string][]models.App {
		users := map[string][]models.App{}
		for _, p := range profiles {
			for _, id := range ids(p.linkages) {
				if !slices.ContainsFunc(users[id], func(a models.App) bool { return a.ID == p.app.ID }) {
					users[id] = append(users[id], p.app)
				}
			}
		}
		return users
	}
	referencedFile := func(apps []models.App) *file {
		if len(apps) == 1 {
			return signingFile(apps[0])
		}
		return e.file("signing.tf")
	}

	if e.wants(PartCertificates) && len(profiles) > 0 {
		users := usedBy(func(l *apple.ProfileLinkages) []string { return l.CertificateIDs })
		if certificates, ok, err := e.readCertificates(); err != nil {
			return err
		} else if ok {
			for _, certificate := range certificates {
				if apps, used := users[certificate.ID]; used {
					e.exportCertificate(referencedFile(apps), certificate)
				}
			}
		}
	}

	switch {
	case e.named(PartDevices):
		// Named, devices are the account part they are without --app: every
		// enabled device, as a resource.
		if devices, ok, err := e.readDevices(); err != nil {
			return err
		} else if ok {
			for _, device := range devices {
				e.exportDevice(e.file("signing.tf"), device)
			}
		}
	case e.wants(PartDevices) && len(profiles) > 0:
		users := usedBy(func(l *apple.ProfileLinkages) []string { return l.DeviceIDs })
		if len(users) == 0 {
			break
		}
		if devices, ok, err := e.readDevices(); err != nil {
			return err
		} else if ok {
			for _, device := range devices {
				if apps, used := users[device.ID]; used {
					e.referenceDevice(referencedFile(apps), device)
				}
			}
		}
	}

	if err := e.exportMerchantAndPassTypeIDs(e.file("signing.tf")); err != nil {
		return err
	}

	if e.wants(PartProfiles) {
		for _, p := range profiles {
			e.exportProfile(signingFile(p.app), p.profile, p.linkages)
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

// referenceDevice reads a device a profile uses through a data source, for an
// export scoped to some apps: the device belongs to the account, and managing
// it from one app's configuration would let that configuration rename or
// disable it for every other.
func (e *exporter) referenceDevice(f *file, device models.Device) {
	if device.Attributes.Status == models.DeviceStatusDISABLED {
		e.note(Skipped, "", "Device %q (%s) is disabled, so it was left out.", device.Attributes.Name, device.Attributes.UDID)
		return
	}
	b, _ := e.dataSource(f, "apple_devices", device.ID, device.Attributes.Name)
	b.str("udid_pattern", "^"+regexp.QuoteMeta(device.Attributes.UDID)+"$")
}

// exportCertificate writes a data source, never a resource. Apple does not
// return the CSR a certificate was issued from, and the export has no other
// source for one, so an apple_certificate written here would have no
// csr_content to give it: the configuration would not validate, and any CSR
// from another key would reissue the certificate every machine is signing
// with, revoking the original. Handing it over is the user's step, with a CSR
// from the key they hold; the note says how.
func (e *exporter) exportCertificate(f *file, certificate models.Certificate) {
	b, addr := e.dataSource(f, "apple_certificates", certificate.ID,
		certificate.Attributes.DisplayName, string(certificate.Attributes.CertificateType))
	b.str("serial_number", certificate.Attributes.SerialNumber)

	e.note(Attention, addr.String(),
		"Certificate %q (serial %s) is read through a data source, so Terraform will not renew it: early_renewal_hours acts only on an apple_certificate Terraform owns. "+
			"To hand it over (provider 0.8.2 or later), import it as an apple_certificate with id %q and csr_content set to a CSR generated from the private key it was issued for, "+
			"check that the plan updates it in place rather than replacing it (a replacement revokes it), then point the profiles that use it at the resource's id instead of %s.certificates[0].id. "+
			"The Code Signing guide walks through it.",
		certificate.Attributes.DisplayName, certificate.Attributes.SerialNumber, certificate.Attributes.SerialNumber, addr.String())
}

// exportableProfile is false for a profile the export leaves out: one that is
// no longer active, or has no type. noted says whether to say so, which an
// export scoped to some apps cannot, since it has not yet read which app the
// profile was for.
func (e *exporter) exportableProfile(profile models.Profile, noted bool) bool {
	name := profile.Attributes.Name
	if profile.Attributes.ProfileState != nil && *profile.Attributes.ProfileState != models.ProfileStateActive {
		if noted {
			e.note(Skipped, "", "Profile %q is %s, so it was left out.", name, *profile.Attributes.ProfileState)
		}
		return false
	}
	if profile.Attributes.ProfileType == nil {
		if noted {
			e.note(Skipped, "", "Profile %q has no type Apple reports, so it was left out.", name)
		}
		return false
	}
	return true
}

func (e *exporter) exportProfile(f *file, profile models.Profile, linkages *apple.ProfileLinkages) {
	name := profile.Attributes.Name
	b, _ := e.resource(f, "apple_profile", profile.ID, profile.ID, name)
	b.str("name", name)
	b.str("profile_type", string(*profile.Attributes.ProfileType))
	b.set("bundle_id", e.idOf("apple_bundle_id", linkages.BundleID))

	certificates := make([]value, 0, len(linkages.CertificateIDs))
	for _, id := range linkages.CertificateIDs {
		certificates = append(certificates, e.firstOf(id, "apple_certificates"))
	}
	b.refs("certificates", certificates)

	if len(linkages.DeviceIDs) > 0 {
		devices := make([]value, 0, len(linkages.DeviceIDs))
		for _, id := range linkages.DeviceIDs {
			if addr, ok := e.addresses["apple_device/"+id]; ok {
				devices = append(devices, ref(addr.traversal("id")))
			} else {
				devices = append(devices, e.firstOf(id, "apple_devices"))
			}
		}
		b.refs("devices", devices)
	}
}

// firstOf refers to the one record a listing data source was written to find,
// and otherwise writes Apple's ID as it is.
func (e *exporter) firstOf(appleID, dataType string) value {
	if addr, ok := e.addresses["data."+dataType+"/"+appleID]; ok {
		// The listing attribute is named after the data source:
		// apple_certificates has certificates, apple_devices has devices.
		return ref(addr.traversal(strings.TrimPrefix(dataType, "apple_"), 0, "id"))
	}
	return literal(cty.StringVal(appleID))
}
