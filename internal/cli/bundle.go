// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"howett.net/plist"
)

// builtApp is the app inside an .ipa, an .xcarchive or a bare .app: its
// Info.plist, and its executable for whatever reads the signature.
type builtApp struct {
	info map[string]any

	// dir is the .app on disk, or the .app's prefix inside the .ipa's zip.
	dir string
	// macOS bundles keep Info.plist and the executable under Contents/.
	macOS bool
	zip   *zip.ReadCloser
}

var ipaInfoPlist = regexp.MustCompile(`^Payload/[^/]+\.app/Info\.plist$`)

// openBuiltApp reads the Info.plist, which is binary in anything Xcode built,
// so it works on any runner. The .ipa is read in place, not unzipped.
func openBuiltApp(location string) (*builtApp, error) {
	if _, err := os.Stat(location); err != nil {
		return nil, usageError("no file %s", location)
	}

	switch strings.ToLower(filepath.Ext(strings.TrimRight(location, "/"))) {
	case ".ipa":
		return openIPA(location)
	case ".xcarchive":
		apps, _ := filepath.Glob(filepath.Join(location, "Products", "Applications", "*.app"))
		if len(apps) != 1 {
			return nil, fmt.Errorf("%s holds %d apps under Products/Applications, not one", location, len(apps))
		}
		return openAppDir(apps[0])
	case ".app":
		return openAppDir(location)
	}
	return nil, usageError("%s is not an .ipa, an .xcarchive or an .app", location)
}

func openAppDir(dir string) (*builtApp, error) {
	app := &builtApp{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "Info.plist"))
	if errors.Is(err, os.ErrNotExist) {
		app.macOS = true
		data, err = os.ReadFile(filepath.Join(dir, "Contents", "Info.plist"))
	}
	if err != nil {
		return nil, fmt.Errorf("no Info.plist in %s", dir)
	}
	if app.info, err = parsePlist(data); err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	return app, nil
}

func openIPA(location string) (*builtApp, error) {
	archive, err := zip.OpenReader(location)
	if err != nil {
		return nil, fmt.Errorf("%s is not a readable .ipa: %w", location, err)
	}

	var found []*zip.File
	for _, file := range archive.File {
		if ipaInfoPlist.MatchString(file.Name) {
			found = append(found, file)
		}
	}
	if len(found) != 1 {
		_ = archive.Close()
		return nil, fmt.Errorf("%s holds %d apps under Payload/, not one", location, len(found))
	}

	data, err := readZipFile(found[0])
	if err == nil {
		var info map[string]any
		if info, err = parsePlist(data); err == nil {
			return &builtApp{info: info, dir: path.Dir(found[0].Name), zip: archive}, nil
		}
	}
	_ = archive.Close()
	return nil, fmt.Errorf("%s: %w", location, err)
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func parsePlist(data []byte) (map[string]any, error) {
	var values map[string]any
	if _, err := plist.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("unreadable property list: %w", err)
	}
	return values, nil
}

func (b *builtApp) close() {
	if b.zip != nil {
		_ = b.zip.Close()
	}
}

// value is an Info.plist string, or "" when it is absent or not a string.
func (b *builtApp) value(key string) string {
	s, _ := b.info[key].(string)
	return s
}

// executable is the app's main binary on disk. Out of an .ipa it is copied to a
// temporary file, which cleanup removes; nothing else is extracted.
func (b *builtApp) executable() (file string, cleanup func(), err error) {
	name := b.value("CFBundleExecutable")
	if name == "" {
		return "", nil, errors.New("the Info.plist names no CFBundleExecutable")
	}
	if b.macOS {
		name = path.Join("Contents", "MacOS", name)
	}

	if b.zip == nil {
		return filepath.Join(b.dir, filepath.FromSlash(name)), func() {}, nil
	}

	inside := path.Join(b.dir, name)
	for _, entry := range b.zip.File {
		if entry.Name != inside {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return "", nil, err
		}
		defer reader.Close()

		temp, err := os.CreateTemp("", "apple-executable-*")
		if err != nil {
			return "", nil, err
		}
		cleanup := func() { _ = os.Remove(temp.Name()) }
		_, err = io.Copy(temp, reader)
		if closeErr := temp.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			cleanup()
			return "", nil, err
		}
		return temp.Name(), cleanup, nil
	}
	return "", nil, fmt.Errorf("no %s in the .ipa", inside)
}

// codesignEntitlements reads the entitlements a binary was signed with. Only
// codesign can, so this one check needs macOS. A binary signed without any
// reads as none rather than as an error.
func codesignEntitlements(executable string) (map[string]any, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("/usr/bin/codesign", "-d", "--entitlements", "-", "--xml", executable)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codesign could not read the signature: %s", strings.TrimSpace(stderr.String()))
	}
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return map[string]any{}, nil
	}
	return parsePlist(stdout.Bytes())
}
