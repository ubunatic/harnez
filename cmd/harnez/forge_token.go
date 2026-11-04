package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"ubunatic.com/harnez/internal/forgetoken"
	"ubunatic.com/harnez/internal/prwait"
)

func newForgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forge",
		Short: "Manage forge access for harnez's read paths",
	}
	cmd.AddCommand(newForgeTokenCmd())
	return cmd
}

func newForgeTokenCmd() *cobra.Command {
	var host, user string
	var localOnly bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Create, rotate, revoke or show harnez's read-only Forgejo token",
		Long: `Manage one read-only Forgejo access token per forge host (scopes and name prefix
in spec/forge_token.yaml). harnez reads the stored token before CODEBERG_TOKEN/FORGEJO_TOKEN,
so pr wait and the forge tracker mode (find issues, issues show) can read private repos.

Forgejo creates and deletes tokens only with your password (basic auth), never with a token.
create, rotate and revoke prompt for the password, and for a 2FA code, on the terminal; there
is no flag or env var for them, and neither is ever printed or logged.

The token goes into the OS keyring (secret-tool) when one works, else into a 0600 file under
$XDG_CONFIG_HOME/harnez/forge-tokens. Its secret is never printed.`,
	}
	cmd.PersistentFlags().StringVar(&host, "host", "codeberg.org", "forge host from spec/pr.yaml")

	create := &cobra.Command{
		Use: "create", Short: "Create and store a new read-only token", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeToken(cmd, host, user, func(sp forgetoken.Spec, st forgetoken.Store, c forgetoken.Client, l forgetoken.Login) error {
				res, err := forgetoken.Create(sp, st, c, host, l, time.Now())
				if err != nil {
					return err
				}
				printTokenMeta(cmd.OutOrStdout(), "created", res.Meta, st)
				return nil
			})
		},
	}
	create.Flags().StringVar(&user, "user", "", "forge username (prompted when empty)")

	rotate := &cobra.Command{
		Use: "rotate", Short: "Replace the stored token with a new one and revoke the old one", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeToken(cmd, host, "", func(sp forgetoken.Spec, st forgetoken.Store, c forgetoken.Client, l forgetoken.Login) error {
				res, err := forgetoken.Rotate(sp, st, c, host, l, time.Now())
				if err != nil {
					return err
				}
				printTokenMeta(cmd.OutOrStdout(), "rotated", res.Meta, st)
				if !res.OldRevoked {
					return fmt.Errorf("new token stored, but revoking the old token %q failed: %v; delete it in the forge's settings", res.OldName, res.OldErr)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "revoked old token %s\n", res.OldName)
				return nil
			})
		},
	}

	revoke := &cobra.Command{
		Use: "revoke", Short: "Revoke the stored token on the forge and forget it locally", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if localOnly {
				_, st, err := forgeTokenStore()
				if err != nil {
					return err
				}
				m, err := forgetoken.Revoke(st, forgetoken.Client{}, host, forgetoken.Login{}, true)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "forgot %s locally; it is still live on %s until you delete it there\n", m.Name, host)
				return nil
			}
			return runForgeToken(cmd, host, "", func(_ forgetoken.Spec, st forgetoken.Store, c forgetoken.Client, l forgetoken.Login) error {
				m, err := forgetoken.Revoke(st, c, host, l, false)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "revoked %s on %s and removed it locally\n", m.Name, host)
				return nil
			})
		},
	}
	revoke.Flags().BoolVar(&localOnly, "local-only", false, "only forget the token locally; no password needed")

	status := &cobra.Command{
		Use: "status", Short: "Show the stored token's name, scopes and store (never the secret)", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, st, err := forgeTokenStore()
			if err != nil {
				return err
			}
			m, ok, err := st.Meta(host)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "no token stored for %s\n", host)
				return nil
			}
			state := "stored"
			if st.Secret(host) == "" {
				state = "MISSING (metadata present, secret unreadable)"
			}
			printTokenMeta(cmd.OutOrStdout(), state, m, st)
			return nil
		},
	}
	cmd.AddCommand(create, rotate, revoke, status)
	return cmd
}

func forgeTokenStore() (forgetoken.Spec, forgetoken.Store, error) {
	sp, err := forgetoken.LoadSpec()
	if err != nil {
		return sp, forgetoken.Store{}, err
	}
	st, err := forgetoken.SystemStore(sp)
	return sp, st, err
}

func printTokenMeta(w io.Writer, state string, m forgetoken.Meta, st forgetoken.Store) {
	where := "keyring (secret-tool)"
	if m.Store == "file" {
		where = st.Dir + "/" + m.Host + ".token (0600)"
	}
	fmt.Fprintf(w, "%s: %s for %s@%s\n  scopes: %s\n  store:  %s\n  created: %s\n",
		state, m.Name, m.User, m.Host, strings.Join(m.Scopes, ", "), where, m.Created.Format(time.RFC3339))
}

// runForgeToken resolves the forge's API base, opens the store, prompts for the login on the
// terminal and runs op. User comes from the flag, else the stored metadata, else a prompt.
func runForgeToken(cmd *cobra.Command, host, user string, op func(forgetoken.Spec, forgetoken.Store, forgetoken.Client, forgetoken.Login) error) error {
	prSpec, err := prwait.LoadSpec()
	if err != nil {
		return err
	}
	apiBase := ""
	for _, fg := range prSpec.Forges {
		if fg.Host == host && fg.Kind == "forgejo" {
			apiBase = fg.APIBase
		}
	}
	if apiBase == "" {
		return fmt.Errorf("%s is not a Forgejo host in spec/pr.yaml", host)
	}
	sp, st, err := forgeTokenStore()
	if err != nil {
		return err
	}
	if user == "" {
		if m, ok, _ := st.Meta(host); ok {
			user = m.User
		}
	}
	l, err := promptLogin(cmd.ErrOrStderr(), host, user)
	if err != nil {
		return err
	}
	c := forgetoken.Client{HTTP: &http.Client{Timeout: prSpec.RequestTimeout}, APIBase: apiBase}
	return op(sp, st, c, l)
}

// promptLogin reads user, password (no echo) and an optional 2FA code from /dev/tty.
func promptLogin(w io.Writer, host, user string) (forgetoken.Login, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(int(tty.Fd())) {
		return forgetoken.Login{}, fmt.Errorf("a terminal is needed to enter the %s password; run this command yourself", host)
	}
	defer tty.Close()
	rd := bufio.NewReader(tty)
	if user == "" {
		fmt.Fprintf(tty, "%s username: ", host)
		line, err := rd.ReadString('\n')
		if err != nil {
			return forgetoken.Login{}, err
		}
		user = strings.TrimSpace(line)
	}
	if user == "" {
		return forgetoken.Login{}, fmt.Errorf("no username given")
	}
	fmt.Fprintf(tty, "%s password for %s (not shown): ", host, user)
	pw, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return forgetoken.Login{}, err
	}
	if len(pw) == 0 {
		return forgetoken.Login{}, fmt.Errorf("no password given")
	}
	fmt.Fprint(tty, "2FA code (empty if 2FA is off): ")
	otp, err := rd.ReadString('\n')
	if err != nil && err != io.EOF {
		return forgetoken.Login{}, err
	}
	_ = w
	return forgetoken.Login{User: user, Password: string(pw), OTP: strings.TrimSpace(otp)}, nil
}
