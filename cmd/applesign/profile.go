// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProfileDirectories are the directories Xcode reads provisioning profiles
// from, for the given home directory.
//
// The first is the classic location, still read by every Xcode version. Xcode
// 16 moved its own copy to the second, which is only worth creating on a
// machine that actually has Xcode.
func ProfileDirectories(home string) []string {
	directories := []string{
		filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"),
	}

	xcodeData := filepath.Join(home, "Library", "Developer", "Xcode")
	if info, err := os.Stat(xcodeData); err == nil && info.IsDir() {
		directories = append(directories, filepath.Join(xcodeData, "UserData", "Provisioning Profiles"))
	}

	return directories
}

// InstallProfile writes one decoded profile into every directory Xcode reads,
// naming it by UUID the way Xcode itself does.
func InstallProfile(content []byte, uuid string, directories []string) ([]string, error) {
	var written []string

	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return written, fmt.Errorf("creating %s: %w", directory, err)
		}

		path := filepath.Join(directory, uuid+".mobileprovision")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return written, fmt.Errorf("writing %s: %w", path, err)
		}

		written = append(written, directory)
	}

	return written, nil
}
