//go:build ignore

package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-=[]{}|;':\",.<>/?°±×÷≤≥↑↓↔┈┄╭╮╰╯⚠✦⣿"

func main() {
	file, err := os.Open("docs/data/golden-font-5x8-import.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		panic(err)
	}
	var out strings.Builder
	out.WriteString("# yaml-language-server: $schema=../../../spec/schemas/font-glyphs.schema.json\nfonts:\n  retro_pixel_5x8:\n    cell: [6, 8]\n    glyphs:\n")
	for i, r := range []rune(chars) {
		x0, y0 := (i%16)*6, (i/16)*8
		fmt.Fprintf(&out, "      %q:\n", string(r))
		for y := 0; y < 8; y++ {
			row := make([]byte, 6)
			for x := range row {
				_, _, _, a := img.At(x0+x, y0+y).RGBA()
				if a != 0 {
					row[x] = '1'
				} else {
					row[x] = ' '
				}
			}
			fmt.Fprintf(&out, "        - %q\n", string(row))
		}
	}
	path := filepath.Join("internal", "readcard", "spec", "font_5x8.yaml")
	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		panic(err)
	}
	fmt.Println(path)
}
