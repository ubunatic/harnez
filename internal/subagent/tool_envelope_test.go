package subagent

import (
	"testing"
)

func TestParseEnvelope(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		strict  bool
		lenient bool
	}{
		{
			name:    "valid tool",
			input:   `{"tool":"my_tool","args":{"x":1}}`,
			strict:  true,
			lenient: true,
		},
		{
			name:    "valid final",
			input:   `{"final":"done"}`,
			strict:  true,
			lenient: true,
		},
		{
			name:    "malformed json",
			input:   `{"tool":"my_tool"`,
			strict:  false,
			lenient: false,
		},
		{
			name:    "mixed prose and json",
			input:   `Sure, I can do that. {"tool":"my_tool","args":{}} And some text after.`,
			strict:  false,
			lenient: true,
		},
		{
			name:    "missing fields",
			input:   `{"other":"my_tool"}`,
			strict:  false,
			lenient: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errStrict := ParseStrictEnvelope(tc.input)
			if (errStrict == nil) != tc.strict {
				t.Errorf("strict: expected success=%v, got err=%v", tc.strict, errStrict)
			}

			_, errLenient := ParseLenientEnvelope(tc.input)
			if (errLenient == nil) != tc.lenient {
				t.Errorf("lenient: expected success=%v, got err=%v", tc.lenient, errLenient)
			}
		})
	}
}
