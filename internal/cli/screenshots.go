// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newScreenshotsCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screenshots",
		Short: "Take App Store screenshots on a simulator",
	}
	cmd.AddCommand(newScreenshotsCaptureCommand(a))
	return cmd
}

type simulator struct {
	Name        string `json:"name"`
	UDID        string `json:"udid"`
	IsAvailable bool   `json:"isAvailable"`
}

func newScreenshotsCaptureCommand(a *app) *cobra.Command {
	var (
		device string
		out    string
	)

	cmd := &cobra.Command{
		Use:   "capture SCREEN...",
		Short: "Capture one App Store screenshot per SCREEN while you drive the app",
		Long: "Boots the simulator, pins its status bar to 9:41 with a full battery and full bars, and\n" +
			"asks for each SCREEN in turn: put the app on it and press Enter (s skips, q stops). Files\n" +
			"are OUT/<device>/<nn>-<screen>.jpg.\n\n" +
			"They are JPEG because Apple rejects an image with an alpha channel, which simctl's PNGs\n" +
			"have. Shoot on a 6.9\" iPhone and Apple scales every smaller size from it, so one set is a\n" +
			"complete submission. Terraform's apple_app_screenshot_set uploads the files.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageError("name at least one screen, such as: home settings")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, screens []string) error {
			udid, err := findSimulator(device)
			if err != nil {
				return err
			}

			a.logf("Booting %s (%s)…", device, udid)
			if out, err := exec.Command("xcrun", "simctl", "bootstatus", udid, "-b").CombinedOutput(); err != nil {
				return fmt.Errorf("booting %s: %v: %s", device, err, strings.TrimSpace(string(out)))
			}
			_ = exec.Command("open", "-a", "Simulator", "--args", "-CurrentDeviceUDID", udid).Run()

			defer func() { _ = exec.Command("xcrun", "simctl", "status_bar", udid, "clear").Run() }()
			if out, err := exec.Command("xcrun", "simctl", "status_bar", udid, "override",
				"--time", "9:41", "--batteryState", "charged", "--batteryLevel", "100",
				"--cellularMode", "active", "--cellularBars", "4", "--dataNetwork", "wifi",
				"--wifiMode", "active", "--wifiBars", "3").CombinedOutput(); err != nil {
				return fmt.Errorf("pinning the status bar: %v: %s", err, strings.TrimSpace(string(out)))
			}

			dir := filepath.Join(out, strings.ReplaceAll(device, " ", "-"))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			a.logf("Writing to %s\n", dir)

			reader := bufio.NewReader(cmd.InOrStdin())
			taken := []string{}
			for i, screen := range screens {
				fmt.Fprintf(a.errOut, "Put the app on %q, then Enter to capture (s skips, q stops): ", screen)
				reply, err := reader.ReadString('\n')
				reply = strings.ToLower(strings.TrimSpace(reply))
				if err != nil || reply == "q" {
					break
				}
				if reply == "s" {
					continue
				}

				file := filepath.Join(dir, fmt.Sprintf("%02d-%s.jpg", i+1, screen))
				if out, err := exec.Command("xcrun", "simctl", "io", udid, "screenshot", "--type=jpeg", file).CombinedOutput(); err != nil {
					return fmt.Errorf("capturing %s: %v: %s", screen, err, strings.TrimSpace(string(out)))
				}
				a.logf("  → %s  %s", file, imageSize(file))
				taken = append(taken, file)
			}

			a.logf("\n%d screenshot(s) in %s.", len(taken), dir)
			return a.emit(map[string]any{"directory": dir, "files": taken}, strings.Join(taken, "\n"))
		},
	}

	cmd.Flags().StringVar(&device, "device", "iPhone 17 Pro Max", "the simulator to shoot on")
	cmd.Flags().StringVar(&out, "out", "screenshots", "where to write")
	return cmd
}

// findSimulator picks the named device on the newest runtime that has it.
func findSimulator(name string) (string, error) {
	data, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return "", fmt.Errorf("simctl: %w (is Xcode installed?)", err)
	}
	var list struct {
		Devices map[string][]simulator `json:"devices"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return "", err
	}

	runtimes := sortedKeys(list.Devices)
	sort.Sort(sort.Reverse(sort.StringSlice(runtimes)))
	names := map[string]bool{}
	for _, runtime := range runtimes {
		for _, device := range list.Devices[runtime] {
			if device.Name == name && device.IsAvailable {
				return device.UDID, nil
			}
			if strings.HasPrefix(device.Name, "iPhone") {
				names[device.Name] = true
			}
		}
	}
	return "", fmt.Errorf("no simulator named %q; available: %s", name, strings.Join(sortedKeys(names), ", "))
}

func imageSize(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	config, err := jpeg.DecodeConfig(file)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d×%d", config.Width, config.Height)
}
