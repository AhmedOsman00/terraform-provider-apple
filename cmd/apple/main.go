// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Command apple performs the App Store Connect actions Terraform cannot model:
// working out the next version, waiting for a build, installing signing.
package main

import (
	"os"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
