package assess

import (
	"strings"
	"testing"
)

func TestBrailleGlyphBaselineAndMax(t *testing.T) {
	// Baseline at level 1 for both should be ⣀ (0x28C0)
	baseline := BrailleGlyph(1, 1)
	if baseline != '⣀' {
		t.Errorf("BrailleGlyph(1, 1) = %q (%U); want %q (%U)", baseline, baseline, '⣀', '⣀')
	}

	// Max height at level 4 for both should be ⣿ (0x28FF)
	maxRune := BrailleGlyph(4, 4)
	if maxRune != '⣿' {
		t.Errorf("BrailleGlyph(4, 4) = %q (%U); want %q (%U)", maxRune, maxRune, '⣿', '⣿')
	}

	// Zero / level 0 gives 0x2800
	zeroRune := BrailleGlyph(0, 0)
	if zeroRune != 0x2800 {
		t.Errorf("BrailleGlyph(0, 0) = %q (%U); want 0x2800", zeroRune, zeroRune)
	}

	// Single step left dot 7 only, right level 0
	leftLvl1 := BrailleGlyph(1, 0)
	if leftLvl1 != 0x2840 {
		t.Errorf("BrailleGlyph(1, 0) = %q (%U); want 0x2840", leftLvl1, leftLvl1)
	}

	// Right dot 8 only, left level 0
	rightLvl1 := BrailleGlyph(0, 1)
	if rightLvl1 != 0x2880 {
		t.Errorf("BrailleGlyph(0, 1) = %q (%U); want 0x2880", rightLvl1, rightLvl1)
	}
}

func TestResampleSeries(t *testing.T) {
	// Empty input
	res := ResampleSeries(nil, 10)
	if len(res) != 10 {
		t.Fatalf("ResampleSeries(nil, 10) len = %d; want 10", len(res))
	}
	for i, v := range res {
		if v != 0 {
			t.Errorf("res[%d] = %v; want 0", i, v)
		}
	}

	// Single element
	res1 := ResampleSeries([]float64{42}, 10)
	if len(res1) != 10 {
		t.Fatalf("ResampleSeries([42], 10) len = %d; want 10", len(res1))
	}
	for i, v := range res1 {
		if v != 42 {
			t.Errorf("res1[%d] = %v; want 42", i, v)
		}
	}

	// Exact length
	resExact := ResampleSeries([]float64{1, 2, 3, 4}, 4)
	if len(resExact) != 4 || resExact[0] != 1 || resExact[3] != 4 {
		t.Errorf("ResampleSeries exact unexpected: %+v", resExact)
	}

	// Resampling 2 elements to 20 elements (linear ramp from 0 to 19)
	res20 := ResampleSeries([]float64{0, 19}, 20)
	if len(res20) != 20 {
		t.Fatalf("ResampleSeries([0, 19], 20) len = %d; want 20", len(res20))
	}
	if res20[0] != 0 || res20[19] != 19 {
		t.Errorf("res20[0]=%v, res20[19]=%v", res20[0], res20[19])
	}
	if res20[10] != 10 {
		t.Errorf("res20[10]=%v; want 10", res20[10])
	}
}

func TestRenderBrailleSparklineAllZero(t *testing.T) {
	// All-zero series rendering [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀] for 10 cells
	zeros := []float64{0, 0, 0, 0, 0}
	spark := RenderBrailleSparkline(zeros, BrailleOptions{Width: 10})
	want := "⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀"
	if spark != want {
		t.Errorf("RenderBrailleSparkline(zeros) = %q; want %q", spark, want)
	}
}

func TestRenderBrailleSparklineRising(t *testing.T) {
	// A 20-sample rising ramp spanning 0 to 19
	values := make([]float64, 20)
	for i := 0; i < 20; i++ {
		values[i] = float64(i)
	}

	spark := RenderBrailleSparkline(values, BrailleOptions{Width: 10})
	sparkRunes := []rune(spark)
	if len(sparkRunes) != 10 {
		t.Fatalf("spark runes len = %d; want 10 (got %s)", len(sparkRunes), spark)
	}

	// First cell should start at baseline (⣀)
	if sparkRunes[0] != '⣀' {
		t.Errorf("first spark rune = %c; want '⣀'", sparkRunes[0])
	}
	// Last cell should reach ⣿
	if sparkRunes[len(sparkRunes)-1] != '⣿' {
		t.Errorf("last spark rune = %c; want '⣿'", sparkRunes[len(sparkRunes)-1])
	}

	// Rising trend should monotonically increase or stay equal across columns
	want := "⣀⣀⣠⣤⣤⣶⣶⣾⣿⣿"
	if spark != want {
		t.Errorf("RenderBrailleSparkline(rising) = %q; want %q", spark, want)
	}

	// Also verify that an explicitly stepped series [1,1, 1,2, 2,2, 2,3, 3,3, 3,4, 4,4, 4,4, 4,4, 4,4] matches visual progression
	steppedLevels := []float64{1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	steppedSpark := RenderBrailleSparkline(steppedLevels, BrailleOptions{Width: 10, FixedRange: true, Min: 1, Max: 4})
	if steppedSpark != "⣀⣠⣤⣴⣶⣾⣿⣿⣿⣿" {
		t.Errorf("RenderBrailleSparkline(steppedLevels) = %q; want '⣀⣠⣤⣴⣶⣾⣿⣿⣿⣿'", steppedSpark)
	}
}

func TestRenderBrailleSparklineColorTinting(t *testing.T) {
	// Test growth vs reduction vs flat with ANSI color enabled
	// Cell 1: 0 -> 10 (Growth -> Green \x1b[32m)
	// Cell 2: 10 -> 0 (Reduction -> Red \x1b[31m)
	// Cell 3: 5 -> 5 (Flat -> Default / No color escape)
	values := []float64{0, 10, 10, 0, 5, 5}
	spark := RenderBrailleSparkline(values, BrailleOptions{Width: 3, Color: true})

	// Check for green escape
	if !strings.Contains(spark, "\x1b[32m") {
		t.Errorf("spark missing green ANSI: %q", spark)
	}

	// Check for red escape
	if !strings.Contains(spark, "\x1b[31m") {
		t.Errorf("spark missing red ANSI: %q", spark)
	}

	// Check reset code
	if !strings.Contains(spark, "\x1b[0m") {
		t.Errorf("spark missing ANSI reset: %q", spark)
	}
}
