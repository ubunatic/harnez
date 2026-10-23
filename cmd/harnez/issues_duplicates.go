package main

import (
	"errors"

	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/issues"
)

func checkIssueDuplicates(dir, title string) error {
	return issues.CheckDuplicates(dir, title, func(files []issues.IssueFile, title string) ([]issues.DuplicateMatch, error) {
		results, err := find.Similar(files, title)
		if errors.Is(err, find.ErrTooFewSimilarityTokens) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		projected := make([]issues.DuplicateMatch, 0, len(results))
		for _, result := range results {
			projected = append(projected, issues.DuplicateMatch{
				Number: result.Number, RawStatus: result.RawStatus, PlainTitle: result.PlainTitle,
				Score: result.Score, Strong: result.Strong,
			})
		}
		return projected, nil
	})
}
