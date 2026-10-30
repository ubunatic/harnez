// Test stand-in for Pi's terminal cell measurement.
export function visibleWidth(text) {
  let width = 0;
  for (const { segment } of new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(text)) {
    if (/\p{Regional_Indicator}/u.test(segment) || /\p{Extended_Pictographic}/u.test(segment) || /[\u1100-\u115f\u2e80-\ua4cf\uac00-\ud7a3\uf900-\ufaff\ufe10-\ufe6f\uff01-\uff60]/u.test(segment)) width += 2;
    else if (!/^\p{Mark}+$/u.test(segment)) {
      width++;
      width += [...segment].filter((char) => /\p{Spacing_Mark}/u.test(char) && !/[\u1734\u302e\u302f]/u.test(char)).length;
    }
  }
  return width;
}
