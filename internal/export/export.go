// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package export writes Terraform configuration for what an App Store Connect
// account already holds: every resource block carries the value Apple holds,
// beside an import block naming the record, so that the first plan imports
// everything and changes nothing.
//
// Each value is written the way the provider's ImportState and Read record it.
// Anything else -- a value the import leaves null, a list in a different order
// -- shows up in that first plan as a change, and for an attribute that forces
// replacement, as a destroy. Where the provider cannot import a record without
// harm (a certificate is reissued, a screenshot's file is not recoverable) the
// record is left out and the reason goes in the notes rather than the
// configuration.
//
// Like the CLI over it, this package imports internal/apple and nothing from
// internal/provider.
package export

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// Domain is a part of the account that can be exported on its own.
type Domain string

const (
	DomainSigning    Domain = "signing"
	DomainApp        Domain = "app"
	DomainProducts   Domain = "products"
	DomainTestFlight Domain = "testflight"
	DomainTeam       Domain = "team"
)

// Domains lists every domain, in the order they are exported.
var Domains = []Domain{DomainSigning, DomainApp, DomainProducts, DomainTestFlight, DomainTeam}

// ParseDomain accepts a domain name as the CLI takes it.
func ParseDomain(name string) (Domain, error) {
	for _, d := range Domains {
		if string(d) == strings.ToLower(strings.TrimSpace(name)) {
			return d, nil
		}
	}
	names := make([]string, 0, len(Domains))
	for _, d := range Domains {
		names = append(names, string(d))
	}
	return "", fmt.Errorf("unknown part %q: expected one of %s", name, strings.Join(names, ", "))
}

// Options selects what is exported.
type Options struct {
	// Apps names the apps to export by App Store Connect ID or bundle
	// identifier. Empty exports every app on the team.
	Apps []string

	// Domains selects the parts of the account. Empty exports all of them.
	Domains []Domain

	// Progress, when set, is told what is being read.
	Progress func(format string, args ...any)
}

// Severity says whether a note needs acting on.
type Severity string

const (
	// Skipped is a record that exists at Apple and is not in the configuration.
	Skipped Severity = "skipped"
	// Attention is a record that was exported but needs a person to look at it.
	Attention Severity = "attention"
)

// Note is something the configuration does not say for itself.
type Note struct {
	Severity Severity `json:"severity"`
	Address  string   `json:"address,omitempty"`
	Message  string   `json:"message"`
}

// Result is the generated configuration.
type Result struct {
	// Files maps a file name to its content.
	Files map[string][]byte `json:"-"`
	// Imports counts the import blocks written, per resource type.
	Imports map[string]int `json:"imports"`
	// Notes lists what was left out or needs attention.
	Notes []Note `json:"notes"`
}

// ImportCount is the number of records the configuration imports.
func (r *Result) ImportCount() int {
	total := 0
	for _, n := range r.Imports {
		total += n
	}
	return total
}

// FileNames lists the generated files in a stable order.
func (r *Result) FileNames() []string {
	names := make([]string, 0, len(r.Files))
	for name := range r.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// errFatal marks an error that has to stop the export rather than become a
// note: a credential failure would otherwise turn into one note per record.
var errFatal = errors.New("fatal")

type exporter struct {
	src  Source
	opts Options

	files     map[string]*file
	labels    labels
	addresses map[string]address
	imports   map[string]int
	notes     []Note
	secrets   []string
}

// Run reads the account and returns the configuration for it.
func Run(src Source, opts Options) (*Result, error) {
	e := &exporter{
		src:       src,
		opts:      opts,
		files:     map[string]*file{},
		addresses: map[string]address{},
		imports:   map[string]int{},
	}

	if e.wants(DomainSigning) {
		if err := e.exportSigning(); err != nil {
			return nil, err
		}
	}

	if e.wants(DomainApp) || e.wants(DomainProducts) || e.wants(DomainTestFlight) {
		apps, err := e.selectApps()
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			if err := e.exportApp(app); err != nil {
				return nil, err
			}
		}
	}

	if e.wants(DomainTeam) {
		if err := e.exportTeam(); err != nil {
			return nil, err
		}
	}

	return e.result(), nil
}

func (e *exporter) wants(domain Domain) bool {
	if len(e.opts.Domains) == 0 {
		return true
	}
	for _, d := range e.opts.Domains {
		if d == domain {
			return true
		}
	}
	return false
}

func (e *exporter) progress(format string, args ...any) {
	if e.opts.Progress != nil {
		e.opts.Progress(format, args...)
	}
}

func (e *exporter) note(severity Severity, addr string, format string, args ...any) {
	e.notes = append(e.notes, Note{Severity: severity, Address: addr, Message: fmt.Sprintf(format, args...)})
}

// failed decides what a failed read means. A credential failure stops the
// export; anything else is noted against what could not be read, and the
// export carries on without it.
func (e *exporter) failed(what string, err error) error {
	message := err.Error()
	if strings.Contains(message, "status 401") {
		return fmt.Errorf("%w: reading %s: %v", errFatal, what, err)
	}
	if strings.Contains(message, "status 403") {
		e.note(Skipped, "", "Could not read %s: the API key's role is not allowed to (%v).", what, err)
		return nil
	}
	e.note(Skipped, "", "Could not read %s, so it is not in the configuration: %v", what, err)
	return nil
}

func isNotFound(err error) bool {
	message := err.Error()
	return strings.Contains(message, "status 404") || strings.Contains(message, "not found")
}

func (e *exporter) file(name string) *file {
	if f, ok := e.files[name]; ok {
		return f
	}
	f := newFile(name)
	e.files[name] = f
	return f
}

// resource writes an import block and the resource it imports into, and
// remembers the address under Apple's ID so later blocks can refer to it.
func (e *exporter) resource(f *file, typeName, appleID, importID string, labelParts ...string) (*block, address) {
	addr := address{typeName: typeName, label: e.labels.next(typeName, labelParts...)}

	body := f.hcl.Body()
	imp := body.AppendNewBlock("import", nil).Body()
	imp.SetAttributeTraversal("to", addr.traversal())
	imp.SetAttributeRaw("id", literal(ctyString(importID)).tokens())

	resource := body.AppendNewBlock("resource", []string{typeName, addr.label})
	body.AppendNewline()

	e.imports[typeName]++
	if appleID != "" {
		e.addresses[typeName+"/"+appleID] = addr
	}

	return &block{body: resource.Body()}, addr
}

// dataSource writes a data block, remembering its address under Apple's ID.
func (e *exporter) dataSource(f *file, typeName, appleID string, labelParts ...string) (*block, address) {
	addr := address{data: true, typeName: typeName, label: e.labels.next("data."+typeName, labelParts...)}

	body := f.hcl.Body()
	data := body.AppendNewBlock("data", []string{typeName, addr.label})
	body.AppendNewline()

	if appleID != "" {
		e.addresses["data."+typeName+"/"+appleID] = addr
	}

	return &block{body: data.Body()}, addr
}

// idOf refers to the ID of an exported resource when there is one, and
// otherwise writes Apple's ID as it is.
func (e *exporter) idOf(typeName, appleID string) value {
	if addr, ok := e.addresses[typeName+"/"+appleID]; ok {
		return ref(addr.traversal("id"))
	}
	return literal(ctyString(appleID))
}

// secret declares a sensitive variable for a value Apple does not return, and
// returns a reference to it. Its default is null, which is what the import
// records, so the first plan stays empty until somebody sets it.
func (e *exporter) secret(name string) value {
	e.secrets = append(e.secrets, name)
	return ref(hcl.Traversal{hcl.TraverseRoot{Name: "var"}, hcl.TraverseAttr{Name: name}})
}

func (e *exporter) result() *Result {
	result := &Result{
		Files:   map[string][]byte{},
		Imports: e.imports,
		Notes:   e.notes,
	}
	if result.Notes == nil {
		result.Notes = []Note{}
	}

	for name, f := range e.files {
		result.Files[name] = f.Bytes()
	}
	result.Files["providers.tf"] = providersFile().Bytes()
	if len(e.secrets) > 0 {
		result.Files["variables.tf"] = variablesFile(e.secrets).Bytes()
	}
	result.Files[NotesFile] = renderNotes(result)

	return result
}
