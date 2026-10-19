## apple release submit

Submit VERSION for App Review

### Synopsis

Submits the App Store version VERSION for review, the way "Add for Review" and
"Submit to App Review" do in App Store Connect. Terraform prepares the version and
attaches its build; submitting is an event rather than state, so it is done here.

Before anything is sent it checks that VERSION can be submitted and that its build is
attached and VALID. An open draft submission is reused, since Apple allows one per
platform, and a submission App Review left with unresolved issues is resubmitted rather
than replaced. Apple checks the version when it is added, and every problem it reports is
printed.

A version already waiting for or in review is reported, not resubmitted, so a pipeline
can run this again safely.

```
apple release submit VERSION [flags]
```

### Options

```
      --bundle-id string   the app's bundle ID (env APPLE_BUNDLE_ID)
  -h, --help               help for submit
      --platform string    IOS, MAC_OS, TV_OS or VISION_OS (env APPLE_PLATFORM) (default "IOS")
```

### Options inherited from parent commands

```
      --json   print the result as JSON (env APPLE_JSON)
```

### SEE ALSO

* [apple release](apple_release.md)	 - Check an App Store release before it starts, and submit it for review

