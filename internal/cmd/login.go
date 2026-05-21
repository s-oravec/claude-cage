package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/auth"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/oidcdevice"
	"github.com/s-oravec/claude-cage/internal/registry"
)

// NewLoginCmd returns the cobra command for `cage login`.
func NewLoginCmd() *cobra.Command {
	var tokenStdin bool
	var list bool

	c := &cobra.Command{
		Use:   "login [host]",
		Short: "Log in to a cage-hub registry",
		Long: `Authenticate against a cage-hub registry host.

Without --token-stdin, runs the OAuth 2.0 device authorization flow:
prints a URL + user code; the user opens the URL, enters the code,
and cage stores the resulting token in ~/.claude-cage/auth.yaml.

With --token-stdin, reads a Personal Access Token (PAT, format
cgh_<base64url>) from stdin - useful for CI:

    echo "$CAGE_HUB_TOKEN" | cage login cage-hub.io --token-stdin

Use --list to print the registries you are currently logged into.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return runLoginList(cmd.OutOrStdout())
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: cage login <host>  (or --list)")
			}
			return printAPIErrorHint(runLogin(cmd.OutOrStdout(), cmd.InOrStdin(), args[0], tokenStdin))
		},
	}
	c.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read a PAT from stdin (non-interactive)")
	c.Flags().BoolVar(&list, "list", false, "List logged-in registries")
	return c
}

func runLoginList(out io.Writer) error {
	a, err := auth.Load()
	if err != nil {
		return err
	}
	if len(a.Registries) == 0 {
		fmt.Fprintln(out, "no registries")
		return nil
	}
	fmt.Fprintf(out, "%-30s %-20s %s\n", "HOST", "USERNAME", "OBTAINED")
	for host, e := range a.Registries {
		fmt.Fprintf(out, "%-30s %-20s %s\n", host, e.Username, e.ObtainedAt)
	}
	return nil
}

func runLogin(out io.Writer, in io.Reader, host string, tokenStdin bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if tokenStdin {
		token, err := readTokenFromStdin(in)
		if err != nil {
			return err
		}
		if err := auth.AddHost(host, token, ""); err != nil {
			return err
		}
		fmt.Fprintf(out, "Stored token for %s.\n", host)
		return nil
	}

	rc, err := registry.NewClient(host, registry.Options{Insecure: cfg.IsInsecureRegistry(host)})
	if err != nil {
		return err
	}
	info, err := rc.AuthInfo()
	if err != nil {
		return err
	}
	dev, err := oidcdevice.RequestDevice(info.DeviceAuthorizationEndpoint, info.ClientID, info.Scopes)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Open %s and enter code: %s\n", dev.VerificationURI, dev.UserCode)
	fmt.Fprintln(out, "Waiting for authorization...")

	tok, err := oidcdevice.PollToken(info.TokenEndpoint, info.ClientID, dev.DeviceCode, dev.Interval, dev.ExpiresIn)
	if err != nil {
		return err
	}
	var expiresAt time.Time
	if tok.ExpiresIn > 0 {
		expiresAt = time.Now().Add(tok.ExpiresIn)
	}
	if err := auth.AddHostFull(host, tok.AccessToken, tok.RefreshToken, "", expiresAt); err != nil {
		return err
	}
	fmt.Fprintf(out, "Logged in to %s.\n", host)
	return nil
}

func readTokenFromStdin(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("empty token on stdin")
	}
	return line, nil
}
