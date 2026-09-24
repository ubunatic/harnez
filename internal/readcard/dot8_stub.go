//go:build !dot8

package readcard

import "fmt"

func dot8RenderFileToCards(lines []string, filename string, opts RenderOptions) (*RenderResult, error) {
	return nil, fmt.Errorf("dot8 rendering is disabled in this build; rebuild with -tags dot8")
}

// Dot8Encode stub returns text as-is when dot8 tag is omitted.
func Dot8Encode(text string) string {
	return text
}

// Dot8Decode stub returns error when dot8 tag is omitted.
func Dot8Decode(text string) (string, error) {
	return "", fmt.Errorf("dot8 decoding is disabled in this build; rebuild with -tags dot8")
}

// Dot8RoundTripCheck stub returns error when dot8 tag is omitted.
func Dot8RoundTripCheck(text string) error {
	return fmt.Errorf("dot8 is disabled in this build; rebuild with -tags dot8")
}

// Dot8CheckDocument stub returns error when dot8 tag is omitted.
func Dot8CheckDocument(source string) []string {
	return []string{"dot8 is disabled in this build; rebuild with -tags dot8"}
}
