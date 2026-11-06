package usage

import "fmt"

// buildStrataBox renders the Strata TPS box (issue 1182): live tokens/s while
// generating, else the last request's value dimmed with "idle".
func buildStrataBox(width int, st StrataStatus) wbox {
	line := fmt.Sprintf("%s %.1f tok/s", st.Label, st.TPS)
	if st.Prefill {
		line = fmt.Sprintf("%s reading prompt", st.Label)
		if st.PromptTotal > 0 {
			line += fmt.Sprintf(" %d/%d", st.PromptRead, st.PromptTotal)
		}
		if st.TPS > 0 {
			line += fmt.Sprintf(" %.0f tok/s", st.TPS)
		}
	} else if st.Idle {
		line = ansiDimGrey + line + " idle\x1b[0m"
	}
	return wbox{title: "Strata", lines: []string{line}, width: width}
}
