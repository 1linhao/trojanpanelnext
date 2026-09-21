package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"
)

const passwordEnv = "TP_NODE_BUNDLE_PASSWORD"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "node-bundle: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		usage()
		return nil
	}
	switch args[0] {
	case "create":
		set := flag.NewFlagSet("node-bundle create", flag.ContinueOnError)
		set.SetOutput(os.Stderr)
		var options createOptions
		set.StringVar(&options.CredentialPath, "credential-file", "", "Issue #5 Node credential JSON")
		set.StringVar(&options.ConfigPath, "node-config", "", "Node deployment YAML")
		set.StringVar(&options.ClientCAPath, "client-ca", "", "public Web control-plane client CA")
		set.StringVar(&options.OutputPath, "output", "", "encrypted .age output")
		if err := set.Parse(args[1:]); err != nil || len(set.Args()) != 0 {
			return errors.New("invalid create arguments")
		}
		if options.CredentialPath == "" || options.ConfigPath == "" || options.ClientCAPath == "" || options.OutputPath == "" {
			return errors.New("create requires --credential-file, --node-config, --client-ca, and --output")
		}
		password, err := readPassword(true)
		if err != nil {
			return err
		}
		defer wipe(password)
		if err = createBundle(options, password); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Encrypted Node bootstrap bundle written to: %s\n", options.OutputPath)
		return nil
	case "extract", "inspect":
		set := flag.NewFlagSet("node-bundle "+args[0], flag.ContinueOnError)
		set.SetOutput(os.Stderr)
		var bundle, directory string
		set.StringVar(&bundle, "bundle", "", "encrypted Node bootstrap bundle")
		set.StringVar(&directory, "directory", "", "private extraction directory")
		if err := set.Parse(args[1:]); err != nil || len(set.Args()) != 0 || bundle == "" {
			return errors.New(args[0] + " requires --bundle")
		}
		password, err := readPassword(false)
		if err != nil {
			return err
		}
		defer wipe(password)
		if args[0] == "extract" {
			if directory == "" {
				return errors.New("extract requires --directory")
			}
			return extractBundle(bundle, directory, password)
		}
		_, manifest, err := decryptAndValidate(bundle, password)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(manifest)
	default:
		return fmt.Errorf("unsupported command %q", args[0])
	}
}

func readPassword(confirm bool) ([]byte, error) {
	if value, present := os.LookupEnv(passwordEnv); present {
		password := []byte(value)
		if err := validatePassword(password); err != nil {
			wipe(password)
			return nil, err
		}
		return password, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("%s is unset and standard input is not a terminal", passwordEnv)
	}
	fmt.Fprint(os.Stderr, "Node bootstrap bundle password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm Node bootstrap bundle password: ")
		confirmation, confirmErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if confirmErr != nil {
			wipe(password)
			return nil, confirmErr
		}
		defer wipe(confirmation)
		if string(password) != string(confirmation) {
			wipe(password)
			return nil, errors.New("Node bootstrap bundle passwords do not match")
		}
	}
	if err = validatePassword(password); err != nil {
		wipe(password)
		return nil, err
	}
	return password, nil
}

func wipe(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func usage() {
	fmt.Fprintln(os.Stdout, `Usage:
  node-bundle create --credential-file <0600-json> --node-config <0600-yaml> --client-ca <certificate> --output <bundle.age>
  node-bundle extract --bundle <bundle.age> --directory <empty-0700-directory>
  node-bundle inspect --bundle <bundle.age>

The password is read from a terminal by default. Set TP_NODE_BUNDLE_PASSWORD
for non-interactive operation; the password is never accepted as an argument.`)
}
