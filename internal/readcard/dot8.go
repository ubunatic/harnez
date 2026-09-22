package readcard

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	brailleBase    = 0x2800
	dotMask7uint8  = uint8(1 << 6) // Dot 7 for uppercase
	dotMask8uint8  = uint8(1 << 7) // Dot 8 for digit prefix / escape
	dotMask7       = 1 << 6        // Dot 7 for uppercase (as int for brailleEscape calc)
	dotMask8       = 1 << 7        // Dot 8 for digit prefix / escape (as int for brailleEscape calc)
	brailleEscape  = brailleBase + dotMask7 + dotMask8
	brailleMaxCell = 0xFF
)

// letterMasks defines the six-dot Braille patterns for a-z (standard English Braille).
var letterMasks = map[rune]uint8{
	'a': 0b000001, 'b': 0b000011, 'c': 0b001001, 'd': 0b011001,
	'e': 0b010001, 'f': 0b001011, 'g': 0b011011, 'h': 0b010011,
	'i': 0b001010, 'j': 0b011010, 'k': 0b000101, 'l': 0b000111,
	'm': 0b001101, 'n': 0b011101, 'o': 0b010101, 'p': 0b001111,
	'q': 0b011111, 'r': 0b010111, 's': 0b001110, 't': 0b011110,
	'u': 0b100101, 'v': 0b100111, 'w': 0b111010, 'x': 0b101101,
	'y': 0b111101, 'z': 0b110101,
}

// digitLetters maps digits 1-0 to letters a-j for the digit prefix encoding.
var digitLetters = map[rune]rune{
	'1': 'a', '2': 'b', '3': 'c', '4': 'd', '5': 'e',
	'6': 'f', '7': 'g', '8': 'h', '9': 'i', '0': 'j',
}

// reverseDigitLetters reverses the digit mapping.
var reverseDigitLetters = map[rune]rune{
	'a': '1', 'b': '2', 'c': '3', 'd': '4', 'e': '5',
	'f': '6', 'g': '7', 'h': '8', 'i': '9', 'j': '0',
}

// braille8 encodes a single character to its 8-dot Braille representation.
func braille8(ch rune) rune {
	// Handle digits with dot 8 prefix
	if _, ok := digitLetters[ch]; ok {
		return rune(brailleBase + dotMask8)
	}

	// Look up the character in the lowercase map
	lower := strings.ToLower(string(ch))[0]
	mask, ok := letterMasks[rune(lower)]
	if !ok {
		// Character not in alphabet, return unchanged
		return ch
	}

	// Add dot 7 for uppercase letters
	if ch >= 'A' && ch <= 'Z' {
		mask |= dotMask7uint8
	}

	return rune(brailleBase + int(mask))
}

// Dot8Encode converts text to 8-dot Braille encoding.
// Letters become Braille cells, digits get a dot-8 prefix, other characters are copied.
// Existing Braille cells (U+2800-U+28FF) are escaped with the dot-7-and-8 cell.
func Dot8Encode(text string) string {
	var result strings.Builder
	for _, ch := range text {
		// Escape existing Braille cells
		if ch >= 0x2800 && ch <= 0x2800+0xFF {
			result.WriteRune(rune(brailleEscape))
			result.WriteRune(ch)
		} else {
			// Encode digit
			if digitLetter, ok := digitLetters[ch]; ok {
				result.WriteRune(rune(brailleBase + dotMask8))
				// Encode the digit letter
				if mask, ok := letterMasks[digitLetter]; ok {
					result.WriteRune(rune(brailleBase + int(mask)))
				}
			} else {
				// Encode letter
				lower := rune(strings.ToLower(string(ch))[0])
				if mask, ok := letterMasks[lower]; ok {
					maskVal := mask
					if ch >= 'A' && ch <= 'Z' {
						maskVal |= dotMask7uint8
					}
					result.WriteRune(rune(brailleBase + int(maskVal)))
				} else {
					// Character not in alphabet, copy unchanged
					result.WriteRune(ch)
				}
			}
		}
	}
	return result.String()
}

// Dot8Decode converts 8-dot Braille encoded text back to the original.
// Handles digit prefixes, uppercase markers, and escaped Braille cells.
func Dot8Decode(text string) (string, error) {
	var result strings.Builder
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Handle escape sequences: dot-7-and-8 followed by a Braille cell
		if ch == rune(brailleEscape) {
			if i+1 >= len(runes) {
				return "", fmt.Errorf("unterminated Braille escape at end of input")
			}
			escaped := runes[i+1]
			if escaped < 0x2800 || escaped > 0x2800+0xFF {
				return "", fmt.Errorf("Braille escape must be followed by a Braille cell at position %d", i)
			}
			result.WriteRune(escaped)
			i += 2
			continue
		}

		// Handle regular Braille cells
		if ch >= 0x2800 && ch <= 0x2800+0xFF {
			mask := uint8(ch - 0x2800)

			// Handle digit prefix (dot 8 only)
			if (mask & dotMask8uint8) != 0 {
				if mask != dotMask8uint8 {
					return "", fmt.Errorf("invalid dot-8 sequence at position %d", i)
				}
				if i+1 >= len(runes) {
					return "", fmt.Errorf("dot-8 digit prefix must be followed by a Braille cell at position %d", i)
				}

				digitCell := runes[i+1]
				digitMask := uint8(digitCell - 0x2800)
				digitLetter := rune(0)

				// Find which letter this mask corresponds to
				for letter, letterMask := range letterMasks {
					if letterMask == (digitMask & ^dotMask7uint8) {
						digitLetter = letter
						break
					}
				}

				if digitLetter == 0 {
					return "", fmt.Errorf("dot-8 digit prefix must be followed by a-j at position %d", i)
				}

				// Convert letter back to digit
				if digit, ok := reverseDigitLetters[digitLetter]; ok {
					result.WriteRune(digit)
				} else {
					return "", fmt.Errorf("invalid digit letter %c at position %d", digitLetter, i)
				}

				i += 2
				continue
			}

			// Handle regular letter (with or without dot 7 for uppercase)
			uppercase := (mask & dotMask7uint8) != 0
			letterMask := mask & ^dotMask7uint8

			// Find the letter for this mask
			var letter rune
			for l, m := range letterMasks {
				if m == letterMask {
					letter = l
					break
				}
			}

			if letter == 0 {
				return "", fmt.Errorf("unknown 8-dot letter cell at position %d", i)
			}

			if uppercase {
				result.WriteRune(letter - 'a' + 'A')
			} else {
				result.WriteRune(letter)
			}

			i++
			continue
		}

		// Non-Braille character, copy unchanged
		result.WriteRune(ch)
		i++
	}

	return result.String(), nil
}

// Dot8RoundTripCheck verifies that text can be encoded and decoded losslessly.
func Dot8RoundTripCheck(text string) error {
	encoded := Dot8Encode(text)
	decoded, err := Dot8Decode(encoded)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}
	if decoded != text {
		return fmt.Errorf("round-trip failed: decoded text differs from original")
	}
	return nil
}

// Dot8CheckDocument validates round-trip safety section by section (by heading).
func Dot8CheckDocument(source string) []string {
	var errors []string
	for heading, section := range dot8DocumentSections(source) {
		if err := Dot8RoundTripCheck(section); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", heading, err))
		}
	}
	return errors
}

// dot8DocumentSections splits Markdown into heading-led sections for focused checks.
func dot8DocumentSections(text string) map[string]string {
	headingPattern := regexp.MustCompile(`(?m)^#{1,6} .*$`)
	matches := headingPattern.FindAllStringIndex(text, -1)

	sections := make(map[string]string)

	if len(matches) == 0 {
		sections["(document)"] = text
		return sections
	}

	if matches[0][0] > 0 {
		sections["(preamble)"] = text[:matches[0][0]]
	}

	for idx, match := range matches {
		heading := text[match[0]:match[1]]
		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
		}
		sections[heading] = text[start:end]
	}

	return sections
}
