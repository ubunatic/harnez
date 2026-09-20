#!/usr/bin/env python3
"""Naively render Latin letters in Markdown as 8-dot Unicode Braille.

This is intentionally not a literary-Braille translator.  Its project
convention uses the standard six-dot alphabet for lowercase letters, adds dot
7 to uppercase letters, and uses dot 8 as a digit prefix followed by a-j for
1-0.  Existing Braille is escaped with the dots-7-and-8 cell so the transform
is reversible.  Punctuation, Markdown syntax, and box art are copied
unchanged.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


BRAILLE_BASE = 0x2800
DOT_7 = 1 << 6
DOT_8 = 1 << 7
BRAILLE_ESCAPE = chr(BRAILLE_BASE + DOT_7 + DOT_8)

# Dot masks for a-z in the standard English Braille alphabet.
LETTER_MASKS = {
    "a": 0b000001,
    "b": 0b000011,
    "c": 0b001001,
    "d": 0b011001,
    "e": 0b010001,
    "f": 0b001011,
    "g": 0b011011,
    "h": 0b010011,
    "i": 0b001010,
    "j": 0b011010,
    "k": 0b000101,
    "l": 0b000111,
    "m": 0b001101,
    "n": 0b011101,
    "o": 0b010101,
    "p": 0b0001111,
    "q": 0b011111,
    "r": 0b010111,
    "s": 0b001110,
    "t": 0b011110,
    "u": 0b100101,
    "v": 0b100111,
    "w": 0b111010,
    "x": 0b101101,
    "y": 0b111101,
    "z": 0b110101,
}

DIGIT_LETTERS = {
    "1": "a",
    "2": "b",
    "3": "c",
    "4": "d",
    "5": "e",
    "6": "f",
    "7": "g",
    "8": "h",
    "9": "i",
    "0": "j",
}


def braille8(character: str) -> str:
    """Return the project's 8-dot representation for one character."""
    digit_letter = DIGIT_LETTERS.get(character)
    if digit_letter is not None:
        return chr(BRAILLE_BASE + DOT_8) + braille8(digit_letter)

    lower = character.lower()
    mask = LETTER_MASKS.get(lower)
    if mask is None:
        return character
    if character.isupper():
        mask |= DOT_7
    return chr(BRAILLE_BASE + mask)


def translate(text: str) -> str:
    result = []
    for character in text:
        if BRAILLE_BASE <= ord(character) <= BRAILLE_BASE + 0xFF:
            result.extend((BRAILLE_ESCAPE, character))
        else:
            result.append(braille8(character))
    return "".join(result)


def translate_back(text: str) -> str:
    """Decode output from translate, including escaped source Braille."""
    result = []
    index = 0
    while index < len(text):
        character = text[index]
        if character == BRAILLE_ESCAPE:
            if index + 1 >= len(text):
                raise ValueError("unterminated Braille escape at end of input")
            escaped = text[index + 1]
            if not BRAILLE_BASE <= ord(escaped) <= BRAILLE_BASE + 0xFF:
                raise ValueError("Braille escape must be followed by a Braille cell")
            result.append(escaped)
            index += 2
            continue

        codepoint = ord(character)
        if not BRAILLE_BASE <= codepoint <= BRAILLE_BASE + 0xFF:
            result.append(character)
            index += 1
            continue

        mask = codepoint - BRAILLE_BASE
        if mask & DOT_8:
            if mask != DOT_8 or index + 1 >= len(text):
                raise ValueError("invalid dot-8 sequence at character %d" % index)
            digit_cell = text[index + 1]
            digit_mask = ord(digit_cell) - BRAILLE_BASE
            digit = next(
                (value for value, letter in DIGIT_LETTERS.items()
                 if LETTER_MASKS[letter] == digit_mask),
                None,
            )
            if digit is None:
                raise ValueError("dot-8 digit prefix must be followed by a-j")
            result.append(digit)
            index += 2
            continue

        uppercase = bool(mask & DOT_7)
        letter_mask = mask & ~DOT_7
        letter = next(
            (letter for letter, value in LETTER_MASKS.items()
             if value == letter_mask),
            None,
        )
        if letter is None:
            raise ValueError("unknown 8-dot letter cell at character %d" % index)
        result.append(letter.upper() if uppercase else letter)
        index += 1
    return "".join(result)


def output_path(input_path: Path) -> Path:
    if input_path.suffix == ".md":
        return input_path.with_name(f"{input_path.stem}.braille.md")
    return input_path.with_name(f"{input_path.name}.braille.md")


def reverse_output_path(input_path: Path) -> Path:
    suffix = ".braille.md"
    if input_path.name.endswith(suffix):
        return input_path.with_name(input_path.name[:-len(suffix)] + ".md")
    return input_path.with_name(f"{input_path.stem}.md")


def sections(text: str) -> list[tuple[str, str]]:
    """Split Markdown into heading-led sections for focused check errors."""
    matches = list(re.finditer(r"(?m)^#{1,6} .*$", text))
    if not matches:
        return [("(document)", text)]
    result = []
    if matches[0].start() > 0:
        result.append(("(preamble)", text[:matches[0].start()]))
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
        result.append((match.group().strip(), text[match.start():end]))
    return result


def check_document(source: str, reverse: bool) -> list[str]:
    errors = []
    for heading, section in sections(source):
        try:
            if reverse:
                restored = translate_back(section)
                if translate(restored) != section:
                    raise ValueError("re-encoding changed the section")
            else:
                encoded = translate(section)
                if translate_back(encoded) != section:
                    raise ValueError("decoding changed the section")
        except (UnicodeError, ValueError) as error:
            errors.append(f"{heading}: {error}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Naively convert Latin letters in Markdown to 8-dot Braille."
    )
    parser.add_argument("input", type=Path, help="input Markdown file")
    parser.add_argument(
        "--reverse",
        action="store_true",
        help="decode a .braille.md file back to Markdown",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="check round-trip safety by section without writing output",
    )
    parser.add_argument(
        "output",
        type=Path,
        nargs="?",
        help="output path (default: INPUT with .braille.md suffix)",
    )
    args = parser.parse_args()

    if args.check:
        source = args.input.read_text(encoding="utf-8")
        errors = check_document(source, args.reverse)
        for error in errors:
            print(error)
        return 1 if errors else 0

    destination = args.output or (
        reverse_output_path(args.input) if args.reverse else output_path(args.input)
    )
    source = args.input.read_text(encoding="utf-8")
    if args.reverse:
        converted = translate_back(source)
        if translate(converted) != source:
            raise ValueError("reverse self-check failed: re-encoding changed input")
    else:
        converted = translate(source)
        if translate_back(converted) != source:
            raise ValueError("self-check failed: decoding changed input")
    destination.write_text(
        converted, encoding="utf-8"
    )
    print(destination)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
