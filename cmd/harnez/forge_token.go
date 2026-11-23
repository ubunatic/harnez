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
		Short: "Manage forge access for harnez's read and write paths",
	}
	cmd.AddCommand(newForgeTokenCmd(), newForgeStatusCmd())
	return cmd
}

// forgeTokenEnv is one forge token run: settings, store, client and the terminal prompter.
type forgeTokenEnv struct {
	spec   forgetoken.Spec
	store  forgetoken.Store
	client forgetoken.Client
	host   string
	login  forgetoken.Login
	tty    tokenTTY
}

type tokenTTY interface {
	close()
	line(string) (string, error)
	secret(string) (string, error)
	login(string, string) (forgetoken.Login, error)
}

var forgeTokenOpenTTY = func(host string) (tokenTTY, error) { return openTTY(host) }
var forgeTokenAPIBase = forgejoAPIBase

func newForgeTokenCmd() *cobra.Command {
	var host, user string
	var localOnly, write bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Create, rotate, revoke or show Forgejo read and write tokens",
		Long: `Manage separate read and issue write Forgejo access tokens per forge host (scopes, name prefix and
default host in spec/forge_token.yaml). harnez reads the stored token before
CODEBERG_TOKEN/FORGEJO_TOKEN, so pr wait and the forge tracker mode (find issues,
issues show) can read private repos.
Use --write to manage the separate issue write token. Issue writes use it first,
then CODEBERG_TOKEN/FORGEJO_TOKEN with the issue write scope, then fj.

Forgejo creates and deletes tokens only with your password (basic auth), never with a token.
create, rotate and revoke prompt for the password, and for a 2FA code, on the terminal; there
is no flag or env var for them, and neither is ever printed or logged. A 2FA code is
single-use, so rotate asks for a second code before it revokes the old token.

The token goes into the OS keyring (secret-tool) when one works, else into a 0600 file under
$XDG_CONFIG_HOME/harnez/forge-tokens. Its secret is never printed.`,
	}
	cmd.PersistentFlags().BoolVar(&write, "write", false, "use the separate forge issue write token")
	cmd.PersistentFlags().StringVar(&host, "host", "", "forge host from spec/pr.yaml (default: default_host in spec/forge_token.yaml)")

	create := &cobra.Command{
		Use: "create", Short: "Create and store a read or write token", Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeToken(host, user, false, write, func(e forgeTokenEnv) error {
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
			return runForgeToken(host, "", true, write, func(e forgeTokenEnv) error {
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
				sp, st, err := forgeTokenStoreFor(write)
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
			return runForgeToken(host, "", true, write, func(e forgeTokenEnv) error {
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
			sp, st, err := forgeTokenStoreFor(write)
			if err != nil {
				return err
			}
			if err := printTokenStatus(cmd.OutOrStdout(), st, orDefault(host, sp.DefaultHost)); err != nil {
				return err
			}
			if !write {
				_, ws, err := forgeTokenStoreFor(true)
				if err != nil {
					return err
				}
				_, ok, err := ws.Meta(orDefault(host, sp.DefaultHost))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "write token: none (harnez forge token create --write)")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "write token:")
					if err := printTokenStatus(cmd.OutOrStdout(), ws, orDefault(host, sp.DefaultHost)); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	var impUser, name, checkRepo, fromVar string
	var removeSource, wizard bool
	imp := &cobra.Command{
		Use:   "import [<file|->]",
		Short: "Store a token copied from the forge's web settings",
		Long: `Read a token as the forge's settings page shows it (only the token, nothing else) and
store it like create does. The token comes from one of:

  (nothing)     a prompt on the terminal; paste the token, it is not shown and never
                reaches the shell history
  -             stdin; a pipe is read as is, a terminal gets the hidden prompt
  <file>        the file's content
  --from NAME   the environment variable NAME, e.g. CODEBERG_TOKEN

Errors name the source, never the token. With --wizard, prompt for all import options and
the token on the terminal (when no <file|-> or --from is given). --user names
the forge account that owns it. --name is the token's name on the forge; without it, revoke
and rotate cannot delete it there. --check owner/repo first reads that repo's issues with
the token and stores nothing on failure. Forgejo lists a token's scopes only with the
password, so the scopes themselves cannot be checked here. The source file is kept unless
--remove-source is given.

  harnez forge token import --user ubunatic --name issue-read --check ubunatic/harnez
  harnez forge token import --write --user ubunatic --from CODEBERG_TOKEN
  harnez forge token import --wizard`,
		Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var file string
			if len(args) > 0 {
				file = args[0]
			}
			if fromVar != "" && file != "" {
				return fmt.Errorf("--from and <file|-> are exclusive; give one token source")
			}
			if removeSource && (file == "" || file == "-") {
				return fmt.Errorf("--remove-source applies only to a <file>")
			}
			return runForgeTokenImport(cmd.OutOrStdout(), cmd.InOrStdin(), importSource{file: file, fromVar: fromVar}, host, impUser, name, checkRepo, removeSource, write, wizard)
		},
	}
	imp.Flags().StringVar(&impUser, "user", "", "forge username that owns the token (required)")
	imp.Flags().StringVar(&name, "name", "", "the token's name on the forge (needed for revoke)")
	imp.Flags().StringVar(&checkRepo, "check", "", "owner/repo whose issues the token must be able to read")
	imp.Flags().BoolVar(&removeSource, "remove-source", false, "delete <file> after the token is stored")
	imp.Flags().BoolVar(&wizard, "wizard", false, "prompt for import options and token on the terminal")
	imp.Flags().StringVar(&fromVar, "from", "", "read the token from environment variable `NAME` (e.g. CODEBERG_TOKEN)")
	cmd.AddCommand(create, rotate, revoke, status, imp)
	return cmd
}

type forgeTokenImportConfig struct {
	store     forgetoken.Store
	apiBase   string
	timeout   time.Duration
	host      string
	user      string
	name      string
	checkRepo string
	data      []byte
}

// importSource says where import reads the token: a file, "-" for stdin, an environment
// variable (fromVar), or, when both are empty, a hidden prompt on the terminal.
type importSource struct {
	file    string
	fromVar string
}

// label names the source in errors; it never contains the token.
func (src importSource) label() string {
	if src.fromVar != "" {
		return "--from " + src.fromVar
	}
	return src.file
}

func runForgeTokenImport(w io.Writer, stdin io.Reader, src importSource, host, user, name, checkRepo string, removeSource, write, wizard bool) error {
	file := src.file
	var cfg forgeTokenImportConfig
	var err error
	if wizard {
		cfg, err = runForgeTokenImportWizard(stdin, src, host, user, name, checkRepo, write)
		if err != nil {
			return err
		}
		if file == "-" {
			removeSource = false
		}
	} else {
		sp, st, err := forgeTokenStoreFor(write)
		if err != nil {
			return err
		}
		host = orDefault(host, sp.DefaultHost)
		apiBase, timeout, err := forgeTokenAPIBase(host)
		if err != nil {
			return err
		}
		if user == "" {
			// Before the prompt, so nobody pastes a token only to be told to start over.
			return fmt.Errorf("--user is required: the forge username that owns the token")
		}
		prompt := importSecretPrompt(sp.Scopes, user, host, write)
		data, rm, err := readImportSource(stdin, src, removeSource, func() (string, error) {
			tty, err := forgeTokenOpenTTY(host)
			if err != nil {
				return "", err
			}
			defer tty.close()
			return tty.secret(prompt)
		})
		if err != nil {
			return err
		}
		removeSource = rm
		cfg = forgeTokenImportConfig{
			store: st, apiBase: apiBase, timeout: timeout,
			host: host, user: user, name: name, checkRepo: checkRepo, data: data,
		}
	}
	tok, err := parseImportedToken(cfg.data)
	if err != nil {
		if src.label() == "" {
			return err
		}
		return fmt.Errorf("%s: %w", src.label(), err)
	}
	if cfg.user == "" {
		return fmt.Errorf("--user is required: the forge username that owns the token")
	}
	c := forgetoken.Client{HTTP: forgetoken.NewHTTPClient(cfg.timeout), APIBase: cfg.apiBase}
	m, err := forgetoken.Import(cfg.store, c, cfg.host, cfg.user, cfg.name, tok, cfg.checkRepo, time.Now())
	if err != nil {
		return err
	}
	printTokenMeta(w, "imported", m, cfg.store)
	if cfg.checkRepo != "" {
		fmt.Fprintf(w, "  check:  read %s issues OK\n", cfg.checkRepo)
	} else {
		fmt.Fprintln(w, "  check:  skipped (use --check owner/repo)")
	}
	if m.Name == "" {
		fmt.Fprintln(w, "  note:   no token name; revoke on the forge by hand, then `revoke --local-only`")
	}
	switch {
	case file == "" || file == "-":
	case removeSource:
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("token stored, but removing %s failed: %w", file, err)
		}
		fmt.Fprintf(w, "removed %s\n", file)
	default:
		fmt.Fprintf(w, "the token is still in %s; delete that file\n", file)
	}
	return nil
}

// readImportSource returns the raw token input and whether the source file may be removed.
// secret prompts on the terminal without echo; it runs when no file or variable is given, or
// for "-" when stdin is a terminal, so a pasted token is never shown.
func readImportSource(stdin io.Reader, src importSource, removeSource bool, secret func() (string, error)) ([]byte, bool, error) {
	switch {
	case src.fromVar != "":
		v := os.Getenv(src.fromVar)
		if strings.TrimSpace(v) == "" {
			return nil, false, fmt.Errorf("--from %s: the variable is not set or empty", src.fromVar)
		}
		return []byte(v), false, nil
	case src.file == "" || (src.file == "-" && importStdinIsTerminal(stdin)):
		raw, err := secret()
		return []byte(raw), false, err
	case src.file == "-":
		data, err := io.ReadAll(io.LimitReader(stdin, 64<<10))
		return data, false, err
	}
	data, err := os.ReadFile(src.file)
	return data, removeSource, err
}

// importStdinIsTerminal reports whether stdin is an interactive terminal, where reading it
// directly would echo the pasted token.
var importStdinIsTerminal = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func importSecretPrompt(scopes []string, user, host string, write bool) string {
	kind := "Token"
	if write {
		kind = "Write token"
	}
	owner := host
	if user != "" {
		owner = user + "@" + host
	}
	return fmt.Sprintf("%s for %s (scopes: %s; paste it, not shown): ", kind, owner, strings.Join(scopes, ", "))
}

// runForgeTokenImportWizard prompts on the terminal for any missing import options and, when
// file is empty, reads the token secret without echo.
func runForgeTokenImportWizard(stdin io.Reader, src importSource, host, user, name, checkRepo string, write bool) (forgeTokenImportConfig, error) {
	var cfg forgeTokenImportConfig
	sp, err := forgetoken.LoadSpec()
	if err != nil {
		return cfg, err
	}
	hostDef := orDefault(host, sp.DefaultHost)
	tty, err := forgeTokenOpenTTY(hostDef)
	if err != nil {
		return cfg, err
	}
	defer tty.close()
	host, err = promptDefault(tty, "Forge host", hostDef)
	if err != nil {
		return cfg, err
	}
	apiBase, timeout, err := forgeTokenAPIBase(host)
	if err != nil {
		return cfg, err
	}
	kindDef := "read"
	if write {
		kindDef = "write"
	}
	kind, err := promptDefault(tty, "Token kind (read/write)", kindDef)
	if err != nil {
		return cfg, err
	}
	switch strings.ToLower(kind) {
	case "read":
		write = false
	case "write":
		write = true
	default:
		return cfg, fmt.Errorf("unknown token kind %q (expected read or write)", kind)
	}
	sp, st, err := forgeTokenStoreFor(write)
	if err != nil {
		return cfg, err
	}
	if _, ok, err := st.Meta(host); err != nil {
		return cfg, err
	} else if ok {
		revokeCmd := "harnez forge token revoke"
		if write {
			revokeCmd += " --write"
		}
		return cfg, fmt.Errorf("a token for %s is already stored; run `%s` first", host, revokeCmd)
	}
	user, err = promptDefault(tty, "Forge username", user)
	if err != nil {
		return cfg, err
	}
	if user == "" {
		return cfg, fmt.Errorf("--user is required: the forge username that owns the token")
	}
	name, err = promptDefault(tty, "Token name on forge (optional, needed for revoke)", name)
	if err != nil {
		return cfg, err
	}
	checkRepo, err = promptDefault(tty, "Check read access on owner/repo (optional)", checkRepo)
	if err != nil {
		return cfg, err
	}
	data, _, err := readImportSource(stdin, src, false, func() (string, error) {
		return tty.secret(fmt.Sprintf("Token for %s@%s (scopes: %s; not shown): ", user, host, strings.Join(sp.Scopes, ", ")))
	})
	if err != nil {
		return cfg, err
	}
	return forgeTokenImportConfig{
		store: st, apiBase: apiBase, timeout: timeout,
		host: host, user: user, name: name, checkRepo: checkRepo, data: data,
	}, nil
}

func promptDefault(tty tokenTTY, label, def string) (string, error) {
	p := label + ": "
	if def != "" {
		p = fmt.Sprintf("%s [%s]: ", label, def)
	}
	ans, err := tty.line(p)
	if err != nil {
		return "", err
	}
	if ans == "" {
		return def, nil
	}
	return ans, nil
}

// parseImportedToken accepts what the forge's web UI hands out: the bare token, alone in the
// input (surrounding whitespace ignored). Errors never echo the input.
func parseImportedToken(data []byte) (string, error) {
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", fmt.Errorf("no token found")
	}
	if strings.ContainsAny(tok, " \t\r\n{}\":") {
		return "", fmt.Errorf("expected only the token, as copied from the forge's settings page")
	}
	return tok, nil
}

func forgejoAPIBase(host string) (string, time.Duration, error) {
	prSpec, err := prwait.LoadSpec()
	if err != nil {
		return "", 0, err
	}
	for _, fg := range prSpec.Forges {
		if fg.Host == host && fg.Kind == "forgejo" {
			return fg.APIBase, prSpec.RequestTimeout, nil
		}
	}
	return "", 0, fmt.Errorf("%s is not a Forgejo host in spec/pr.yaml", host)
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

func forgeTokenStoreFor(write bool) (forgetoken.Spec, forgetoken.Store, error) {
	sp, err := forgetoken.LoadSpec()
	if err != nil {
		return sp, forgetoken.Store{}, err
	}
	if write {
		sp = sp.ForWrite()
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
func runForgeToken(host, user string, needStored, write bool, op func(forgeTokenEnv) error) error {
	sp, st, err := forgeTokenStoreFor(write)
	if err != nil {
		return err
	}
	host = orDefault(host, sp.DefaultHost)
	apiBase, timeout, err := forgeTokenAPIBase(host)
	if err != nil {
		return err
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
	tty, err := forgeTokenOpenTTY(host)
	if err != nil {
		return err
	}
	defer tty.close()
	l, err := tty.login(host, user)
	if err != nil {
		return err
	}
	c := forgetoken.Client{HTTP: forgetoken.NewHTTPClient(timeout), APIBase: apiBase}
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
		return nil, fmt.Errorf("a terminal is needed to enter %s credentials; run this command yourself", host)
	}
	if !term.IsTerminal(int(f.Fd())) {
		f.Close()
		return nil, fmt.Errorf("a terminal is needed to enter %s credentials; run this command yourself", host)
	}
	return &ttyPrompter{f: f, rd: bufio.NewReader(f)}, nil
}

func (t *ttyPrompter) close() { _ = t.f.Close() }

func (t *ttyPrompter) line(prompt string) (string, error) {
	fmt.Fprint(t.f, prompt)
	s, err := t.rd.ReadString('\n')
	if err == io.EOF && s == "" {
		return "", io.EOF
	}
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (t *ttyPrompter) secret(prompt string) (string, error) {
	fmt.Fprint(t.f, prompt)
	b, err := term.ReadPassword(int(t.f.Fd()))
	fmt.Fprintln(t.f)
	if err != nil {
		return "", err
	}
	return string(b), nil
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
