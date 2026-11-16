//go:build !linux

package selfinstall

import (
	"context"
	"fmt"

	"ubunatic.com/harnez/internal/sessioncontract"
)

func promoteStagedOS(context.Context, PromoteOptions, sessioncontract.Contract, []string, string, string) (PromoteResult, error) {
	return PromoteResult{}, fmt.Errorf("host promoter requires Linux O_DIRECTORY|O_NOFOLLOW support")
}
