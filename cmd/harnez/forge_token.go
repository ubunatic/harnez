package main

import (
	"bufio"
	"fmt"
	"io"
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

// forgeTokenEnv is one forge token run: settings, store, client and the terminal prompter.
type forgeTokenEnv struct {
	spec   forgetoken.Spec
	store  forgetoken.Store
	client forgetoken.Client
	host   string
	login  forgetoken.Login
	tty    *ttyPrompter
}

func newForgeTokenCmd() *cobra.Command {
	var host, user string
	var localOnly bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Create, rotate, revoke or show harnez's read-only Forgejo token",
		Long: `Manage one read-only Forgejo access token per forge host (scopes, name prefix and
default host in spec/forge_token.yaml). harnez reads the stored token before
CODEBERG_TOKEN/FORGEJO_TOKEN, so pr wait and the forge tracker mode (find issues,
issues show) can read private repos.

Forgejo creates and deletes tokens only with your password (basic auth), never with a token.
create, rotate and revoke prompt for the password, and for a 2FA code, on the terminal; there
is no flag or env var for them, and neither is ever printed or logged. A 2FA code is
single-use, so rotate asks for a second code before it revokes the old token.

The token goes into the OS keyring (secret-tool) when one works, else into a 0600 file under
$XDG_CONFIG_HOME/harnez/forge-tokens. Its secret is never printed.`,
	}
	cmd.PersistentFlags().StringVar(&host, "host", "", "forge host from spec/pr.yaml (default: default_host in spec/forge_token.yaml)")

	create := &cobra.Command{
		Use: "create", Short: "Create and store a new read-only token", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeToken(host, user, false, func(e forgeTokenEnv) error {
				res, err := forgetoken.Create(e.spec, e.store, e.client, e.host, e.login, time.Now())
				if err != nil {
					return err
				}
				printTokenMeta(cmd.OutOrStdout(), "created", res.Meta, e.store)
				return nil
			})
		},
	}
	create.Flags().StringVar(&user, "user", "", "forge username (prompted when empty)")

	rotate := &cobra.Command{
		Use: "rotate", Short: "Replace the stored token with a new one and revoke the old one", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeToken(host, "", true, func(e forgeTokenEnv) error {
				fresh := func() (string, error) { return e.tty.line("Next 2FA code, to revoke the old token: ") }
				res, err := forgetoken.Rotate(e.spec, e.store, e.client, e.host, e.login, time.Now(), fresh)
				if err != nil {
					return err
				}
				printTokenMeta(cmd.OutOrStdout(), "rotated", res.Meta, e.store)
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
				sp, st, err := forgeTokenStore()
				if err != nil {
					return err
				}
				h := orDefault(host, sp.DefaultHost)
				m, err := forgetoken.Revoke(st, forgetoken.Client{}, h, forgetoken.Login{}, true)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "forgot %s locally; it is still live on %s until you delete it there\n", m.Name, h)
				return nil
			}
			return runForgeToken(host, "", true, func(e forgeTokenEnv) error {
				m, err := forgetoken.Revoke(e.store, e.client, e.host, e.login, false)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "revoked %s on %s and removed it locally\n", m.Name, e.host)
				return nil
			})
		},
	}
	revoke.Flags().BoolVar(&localOnly, "local-only", false, "only forget the token locally; no password needed")

	status := &cobra.Command{
		Use: "status", Short: "Show the stored token's name, scopes and store (never the secret)", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sp, st, err := forgeTokenStore()
			if err != nil {
				return err
			}
			return printTokenStatus(cmd.OutOrStdout(), st, orDefault(host, sp.DefaultHost))
		},
	}
	cmd.AddCommand(create, rotate, revoke, status)
	return cmd
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func forgeTokenStore() (forgetoken.Spec, forgetoken.Store, error) {
	sp, err := forgetoken.LoadSpec()
	if err != nil {
		return sp, forgetoken.Store{}, err
	}
	st, err := forgetoken.SystemStore(sp)
	return sp, st, err
}

func printTokenStatus(w io.Writer, st forgetoken.Store, host string) error {
	m, ok, err := st.Meta(host)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(w, "no token stored for %s\n", host)
		return nil
	}
	state := "stored"
	if st.Secret(host) == "" {
		state = "MISSING (metadata present, secret unreadable)"
	}
	printTokenMeta(w, state, m, st)
	return nil
}

func printTokenMeta(w io.Writer, state string, m forgetoken.Meta, st forgetoken.Store) {
	where := "keyring (secret-tool)"
	if m.Store == "file" {
		where = st.Dir + "/" + m.Host + ".token (0600)"
	}
	fmt.Fprintf(w, "%s: %s for %s@%s\n  scopes: %s\n  store:  %s\n  created: %s\n",
		state, m.Name, m.User, m.Host, strings.Join(m.Scopes, ", "), where, m.Created.Format(time.RFC3339))
}

// runForgeToken resolves the forge's API base, opens the store, checks that a token is stored
// when needStored, prompts for the login on the terminal and runs op. User comes from the flag,
// else the stored metadata, else a prompt.
func runForgeToken(host, user string, needStored bool, op func(forgeTokenEnv) error) error {
	sp, st, err := forgeTokenStore()
	if err != nil {
		return err
	}
	host = orDefault(host, sp.DefaultHost)
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
	m, stored, err := st.Meta(host)
	if err != nil {
		return err
	}
	if needStored && !stored {
		return fmt.Errorf("no token for %s is stored; use `harnez forge token create`", host)
	}
	if user == "" && stored {
		user = m.User
	}
	tty, err := openTTY(host)
	if err != nil {
		return err
	}
	defer tty.close()
	l, err := tty.login(host, user)
	if err != nil {
		return err
	}
	c := forgetoken.Client{HTTP: forgetoken.NewHTTPClient(prSpec.RequestTimeout), APIBase: apiBase}
	return op(forgeTokenEnv{spec: sp, store: st, client: c, host: host, login: l, tty: tty})
}

// ttyPrompter reads answers from /dev/tty. Lines go through one bufio.Reader; the password is
// read with term.ReadPassword on the raw fd. That is safe because a canonical-mode terminal
// returns at most one line per read, so the reader never buffers past the line it was asked for.
type ttyPrompter struct {
	f  *os.File
	rd *bufio.Reader
}

func openTTY(host string) (*ttyPrompter, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("a terminal is needed to enter the %s password; run this command yourself", host)
	}
	if !term.IsTerminal(int(f.Fd())) {
		f.Close()
		return nil, fmt.Errorf("a terminal is needed to enter the %s password; run this command yourself", host)
	}
	return &ttyPrompter{f: f, rd: bufio.NewReader(f)}, nil
}

func (t *ttyPrompter) close() { _ = t.f.Close() }

func (t *ttyPrompter) line(prompt string) (string, error) {
	fmt.Fprint(t.f, prompt)
	s, err := t.rd.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// login asks for user (when empty), password without echo and an optional 2FA code.
func (t *ttyPrompter) login(host, user string) (forgetoken.Login, error) {
	if user == "" {
		u, err := t.line(host + " username: ")
		if err != nil {
			return forgetoken.Login{}, err
		}
		user = u
	}
	if user == "" {
		return forgetoken.Login{}, fmt.Errorf("no username given")
	}
	fmt.Fprintf(t.f, "%s password for %s (not shown): ", host, user)
	pw, err := term.ReadPassword(int(t.f.Fd()))
	fmt.Fprintln(t.f)
	if err != nil {
		return forgetoken.Login{}, err
	}
	if len(pw) == 0 {
		return forgetoken.Login{}, fmt.Errorf("no password given")
	}
	otp, err := t.line("2FA code (empty if 2FA is off): ")
	if err != nil {
		return forgetoken.Login{}, err
	}
	return forgetoken.Login{User: user, Password: string(pw), OTP: otp}, nil
}
