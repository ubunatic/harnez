package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/issues"
)

func checkIssueDuplicates(dir, title string) error {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	files, err := issues.Scan(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("issues new: duplicate scan: %w", err)
	}
	results, err := find.Similar(files, title)
	if errors.Is(err, find.ErrTooFewSimilarityTokens) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("issues new: duplicate scan: %w", err)
	}
	var message strings.Builder
	count := 0
	for _, result := range results {
		if !result.Strong {
			continue
		}
		verb := "open"
		if issues.CanonicalizeStatus(result.RawStatus) == issues.StatusBacklog {
			verb = "promote"
		}
		if count == 0 {
			fmt.Fprintf(&message, "issues new: likely duplicate ticket(s) found (strong-match threshold %d%%):", similarityThresholdPercent())
		}
		fmt.Fprintf(&message, "\n  #%s %s — %s (score %.3f)\n    harnez issues %s %s", result.Number, result.RawStatus, result.PlainTitle, result.Score, verb, result.Number)
		count++
	}
	if count == 0 {
		return nil
	}
	message.WriteString("\nUse --force to create anyway.")
	return errors.New(message.String())
}

func similarityThresholdPercent() int {
	policy, err := issues.DuplicateGateSettings()
	if err != nil {
		return 0
	}
	return int(policy.Threshold * 100)
}
