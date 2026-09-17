// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"testing"
)

func TestGetFont(t *testing.T) {
	for _, name := range []string{"3x5", "5x8", "6x12"} {
		f, err := GetFont(name)
		if err != nil {
			t.Fatalf("expected font %s to exist, got error: %v", name, err)
		}
		if f.CellW <= 0 || f.CellH <= 0 {
			t.Errorf("font %s invalid cell dimensions: %dx%d", name, f.CellW, f.CellH)
		}
		if len(f.Bitmaps) == 0 {
			t.Errorf("font %s has empty bitmaps", name)
		}
	}

	_, err := GetFont("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent font, got nil")
	}
}

func TestRenderText(t *testing.T) {
	fontDef, err := GetFont("5x8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := "if test \"$x\" != \"$y\"; then\n  echo OK\nfi"
	img, cols, rows := RenderText(fontDef, text, 256, 256, 1)

	if img == nil {
		t.Fatal("expected rendered image, got nil")
	}
	if cols <= 0 || rows <= 0 {
		t.Errorf("expected positive cols and rows, got cols=%d rows=%d", cols, rows)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Errorf("expected 256x256 image, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRenderTextScaled(t *testing.T) {
	fontDef, err := GetFont("3x5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := "{ [ ( < > == != := && || ) ] }"
	img, cols, rows := RenderText(fontDef, text, 512, 512, 2)

	if img == nil {
		t.Fatal("expected rendered image, got nil")
	}
	if cols <= 0 || rows <= 0 {
		t.Errorf("expected positive cols and rows, got cols=%d rows=%d", cols, rows)
	}
	if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 512 {
		t.Errorf("expected 512x512 image, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
