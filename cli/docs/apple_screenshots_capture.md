## apple screenshots capture

Capture one App Store screenshot per SCREEN while you drive the app

### Synopsis

Boots the simulator, pins its status bar to 9:41 with a full battery and full bars, and
asks for each SCREEN in turn: put the app on it and press Enter (s skips, q stops). Files
are OUT/<device>/<nn>-<screen>.jpg.

They are JPEG because Apple rejects an image with an alpha channel, which simctl's PNGs
have. Shoot on a 6.9" iPhone and Apple scales every smaller size from it, so one set is a
complete submission. Terraform's apple_app_screenshot_set uploads the files.

```
apple screenshots capture SCREEN... [flags]
```

### Options

```
      --device string   the simulator to shoot on (env APPLE_DEVICE) (default "iPhone 17 Pro Max")
  -h, --help            help for capture
      --out string      where to write (env APPLE_OUT) (default "screenshots")
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple screenshots](apple_screenshots.md)	 - Take App Store screenshots on a simulator

