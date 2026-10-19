// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package cli is the apple command: the actions that go with the provider's
// state. It shares the provider's App Store Connect client and adds nothing to
// Terraform.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// exitError carries the exit code a command wants, for usage errors (2) as
// opposed to failures (1).
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

func usageError(format string, args ...any) error {
	return &exitError{code: 2, err: fmt.Errorf(format, args...)}
}

// app is what every command shares: where to write, and how to reach Apple.
type app struct {
	out    io.Writer
	errOut io.Writer
	json   bool

	newClient func() (*apple.Client, error)
	client    *apple.Client
	clientAt  time.Time
	now       func() time.Time
	sleep     func(time.Duration)

	// entitlements reads a signed binary's entitlements: codesign when nil.
	entitlements func(executable string) (map[string]any, error)
}

// tokenLifetime is how long a client is reused. The provider mints a 20-minute
// token once per client, which outlives any Terraform command but not a wait
// for processing, so a long-running command builds a new client before then.
const tokenLifetime = 15 * time.Minute

// api returns a client whose token has at least five minutes left.
func (a *app) api() (*apple.Client, error) {
	if a.client != nil && a.now().Sub(a.clientAt) < tokenLifetime {
		return a.client, nil
	}

	client, err := a.newClient()
	if err != nil {
		return nil, err
	}

	a.client = client
	a.clientAt = a.now()
	return client, nil
}

func (a *app) logf(format string, args ...any) {
	fmt.Fprintf(a.errOut, format+"\n", args...)
}

// emit writes a result: JSON with --json, otherwise the plain line.
func (a *app) emit(value any, plain string) error {
	if a.json {
		encoder := json.NewEncoder(a.out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}

	_, err := fmt.Fprintln(a.out, plain)
	return err
}

func clientFromEnvironment() (*apple.Client, error) {
	var missing []string
	values := map[string]string{}
	for _, name := range []string{
		"APPLE_APP_STORE_CONNECT_ISSUER_ID",
		"APPLE_APP_STORE_CONNECT_API_KEY",
		"APPLE_APP_STORE_CONNECT_PRIVATE_KEY",
	} {
		values[name] = os.Getenv(name)
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("set %s, the same credentials the provider reads", strings.Join(missing, ", "))
	}

	return apple.NewClient(
		values["APPLE_APP_STORE_CONNECT_ISSUER_ID"],
		values["APPLE_APP_STORE_CONNECT_API_KEY"],
		values["APPLE_APP_STORE_CONNECT_PRIVATE_KEY"],
		nil,
	)
}

func newRootCommand(a *app, version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "apple",
		Short: "App Store Connect actions to go with the Terraform provider",
		Long: "apple performs the steps Terraform cannot model: working out the next version and build,\n" +
			"checking a release before it starts, waiting for a build, installing signing and taking\n" +
			"screenshots. Terraform keeps the state; this does the actions.\n\n" +
			"Credentials are the provider's: APPLE_APP_STORE_CONNECT_ISSUER_ID,\n" +
			"APPLE_APP_STORE_CONNECT_API_KEY and APPLE_APP_STORE_CONNECT_PRIVATE_KEY.\n" +
			"Every flag can also be set as APPLE_<FLAG>, such as APPLE_BUNDLE_ID for --bundle-id.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return applyEnvironment(cmd)
		},
	}
	root.SetOut(a.out)
	root.SetErr(a.errOut)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &exitError{code: 2, err: err}
	})
	root.PersistentFlags().BoolVar(&a.json, "json", false, "print the result as JSON")

	root.AddCommand(
		newVersionCommand(a),
		newReleaseCommand(a),
		newBuildCommand(a),
		newTestFlightCommand(a),
		newSigningCommand(a),
		newScreenshotsCommand(a),
	)
	root.AddCommand(newDocsCommand(root))
	annotateEnvironment(root)

	return root
}

// annotateEnvironment names each flag's variable in its help, so the help and
// the generated reference cannot leave one out.
func annotateEnvironment(cmd *cobra.Command) {
	note := func(flag *pflag.Flag) {
		if flag.Name == "help" || isVersionFlag(cmd, flag) || strings.Contains(flag.Usage, "(env ") {
			return
		}
		flag.Usage += " (env " + envName(flag.Name) + ")"
	}
	cmd.LocalFlags().VisitAll(note)
	cmd.PersistentFlags().VisitAll(note)
	for _, child := range cmd.Commands() {
		annotateEnvironment(child)
	}
}

// envName is the variable that stands in for a flag: --bundle-id is
// APPLE_BUNDLE_ID.
func envName(flag string) string {
	return "APPLE_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
}

// isVersionFlag is the root's own --version. A command's --version, such as
// build verify's, is an ordinary flag with a variable of its own.
func isVersionFlag(cmd *cobra.Command, flag *pflag.Flag) bool {
	return flag.Name == "version" && !cmd.HasParent()
}

// applyEnvironment sets every flag the command line left alone from its
// variable, so CI can set a value once at the workflow level.
func applyEnvironment(cmd *cobra.Command) error {
	var err error
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if err != nil || flag.Changed || flag.Name == "help" || isVersionFlag(cmd, flag) {
			return
		}
		value, ok := os.LookupEnv(envName(flag.Name))
		if !ok || value == "" {
			return
		}
		if setErr := cmd.Flags().Set(flag.Name, value); setErr != nil {
			err = usageError("%s: %v", envName(flag.Name), setErr)
		}
	})
	return err
}

// Execute runs the command line and returns the process exit code.
func Execute(version string) int {
	a := &app{
		out:       os.Stdout,
		errOut:    os.Stderr,
		newClient: clientFromEnvironment,
		now:       time.Now,
		sleep:     time.Sleep,

		entitlements: codesignEntitlements,
	}
	return run(a, version, os.Args[1:])
}

func run(a *app, version string, args []string) int {
	root := newRootCommand(a, version)
	root.SetArgs(args)

	err := root.Execute()
	if err == nil {
		return 0
	}

	fmt.Fprintln(a.errOut, err)
	var exit *exitError
	if errors.As(err, &exit) {
		return exit.code
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	return 1
}

var buildPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return usageError("%s takes %d argument(s), got %d; see --help", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}

func maxArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return usageError("%s takes at most %d argument(s), got %d; see --help", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}
