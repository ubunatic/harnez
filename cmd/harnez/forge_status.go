package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/forgetoken"
	"ubunatic.com/harnez/internal/prwait"
)

func newForgeStatusCmd() *cobra.Command {
	var host, repo string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check forge tokens live and report which token is effective for reads and writes",
		Long: `Probe every configured token source for a forge host (stored read/write tokens,
environment variables in spec/pr.yaml, and gh auth for GitHub) against the live API.
Reports whether each token works and which one is effective for read and write paths.
Token values are never printed.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForgeStatus(cmd, host, repo)
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "forge host from spec/pr.yaml (default: default_host in spec/forge_token.yaml)")
	cmd.Flags().StringVar(&repo, "repo", "", "optional owner/repo to probe issue read access on Forgejo")
	return cmd
}

func runForgeStatus(cmd *cobra.Command, host, repo string) error {
	tokSpec, err := forgetoken.LoadSpec()
	if err != nil {
		return err
	}
	host = orDefault(host, tokSpec.DefaultHost)
	prSpec, err := prwait.LoadSpec()
	if err != nil {
		return err
	}
	var fg forge.Forge
	var found bool
	for _, f := range prSpec.Forges {
		if f.Host == host {
			fg = f
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%s is not a configured forge host in spec/pr.yaml", host)
	}

	opt := forge.StatusOptions{
		Repo:    repo,
		Getenv:  os.Getenv,
		GHToken: forge.SystemTokenSource(os.Getenv).GHToken,
	}
	if fg.Kind == "forgejo" {
		if _, st, err := forgeTokenStoreFor(false); err == nil {
			opt.ReadStored = storedTokenLookup(st)
		}
		if _, ws, err := forgeTokenStoreFor(true); err == nil {
			opt.WriteStored = storedTokenLookup(ws)
		}
	}

	client := forgetoken.NewHTTPClient(prSpec.RequestTimeout)
	rep := forge.CheckStatus(client, fg, opt)
	forge.FormatStatus(cmd.OutOrStdout(), rep)
	return nil
}

func storedTokenLookup(st forgetoken.Store) forge.StoredTokenFunc {
	return func(host string) (string, string, bool) {
		m, ok, err := st.Meta(host)
		if err != nil || !ok {
			return "", "", false
		}
		return st.Secret(host), m.Name, true
	}
}
