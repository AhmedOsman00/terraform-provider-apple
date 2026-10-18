// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	marketingVersionPattern = regexp.MustCompile(`(MARKETING_VERSION = )([^;]+);`)
	projectVersionPattern   = regexp.MustCompile(`(CURRENT_PROJECT_VERSION = )([^;]+);`)
)

// pbxprojPath accepts the .xcodeproj bundle or the project.pbxproj inside it.
func pbxprojPath(project string) (string, error) {
	if project == "" {
		return "", usageError("--project names the .xcodeproj (or APPLE_PROJECT)")
	}
	if strings.HasSuffix(project, ".xcodeproj") {
		project = filepath.Join(project, "project.pbxproj")
	}
	if _, err := os.Stat(project); err != nil {
		return "", fmt.Errorf("no Xcode project at %s", project)
	}
	return project, nil
}

// projectVersions reads the first MARKETING_VERSION and CURRENT_PROJECT_VERSION.
// Every target is expected to carry the same pair; setProjectVersions keeps
// them so.
func projectVersions(project string) (version, build string, err error) {
	path, err := pbxprojPath(project)
	if err != nil {
		return "", "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	if m := marketingVersionPattern.FindSubmatch(content); m != nil {
		version = strings.Trim(string(m[2]), `"`)
	}
	if m := projectVersionPattern.FindSubmatch(content); m != nil {
		build = strings.Trim(string(m[2]), `"`)
	}
	if version == "" {
		return "", "", fmt.Errorf("%s sets no MARKETING_VERSION", path)
	}
	return version, build, nil
}

// setProjectVersions writes the pair onto every build configuration and
// reports how many it changed.
func setProjectVersions(project, version, build string) (int, error) {
	path, err := pbxprojPath(project)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	count := len(marketingVersionPattern.FindAllIndex(content, -1))
	if count == 0 {
		return 0, fmt.Errorf("%s sets no MARKETING_VERSION", path)
	}
	content = marketingVersionPattern.ReplaceAll(content, []byte("${1}"+version+";"))
	content = projectVersionPattern.ReplaceAll(content, []byte("${1}"+build+";"))

	return count, os.WriteFile(path, content, info.Mode().Perm())
}
