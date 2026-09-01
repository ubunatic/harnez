package telemetry

import (
	"testing"
)

func TestScoreShell(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		exitCode  int
		wantScore int
		wantNote  string
	}{
		{
			name:      "Clean success exit 0",
			output:    "PASS\nok  ubunatic.com/harnez 0.45s\n",
			exitCode:  0,
			wantScore: 5,
			wantNote:  "clean success",
		},
		{
			name:      "Go panic runtime error",
			output:    "panic: runtime error: invalid memory address or nil pointer dereference\n[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x12345]\ngoroutine 1 [running]:\nmain.main()",
			exitCode:  2,
			wantScore: 1,
			wantNote:  "go runtime panic / fatal error",
		},
		{
			name:      "Signal termination SIGSEGV (139)",
			output:    "Segmentation fault (core dumped)",
			exitCode:  139,
			wantScore: 1,
			wantNote:  "signal termination (signal 11)",
		},
		{
			name:      "Go build undefined error",
			output:    "./main.go:12:5: undefined: foobar",
			exitCode:  2,
			wantScore: 2,
			wantNote:  "go build / syntax error",
		},
		{
			name:      "Go build type mismatch",
			output:    "./main.go:15:10: cannot use x (variable of type int) as type string in argument to print",
			exitCode:  2,
			wantScore: 2,
			wantNote:  "go build / syntax error",
		},
		{
			name:      "Go test failure",
			output:    "=== RUN   TestSomething\n--- FAIL: TestSomething (0.00s)\n    main_test.go:20: failed\nFAIL",
			exitCode:  1,
			wantScore: 3,
			wantNote:  "go test failure",
		},
		{
			name:      "Python traceback",
			output:    "Traceback (most recent call last):\n  File \"test.py\", line 4, in <module>\n    foo()\nException: something bad",
			exitCode:  1,
			wantScore: 1,
			wantNote:  "python traceback / uncaught exception",
		},
		{
			name:      "Python syntax error",
			output:    "  File \"test.py\", line 2\n    def foo(\n           ^\nSyntaxError: unexpected EOF while parsing",
			exitCode:  1,
			wantScore: 2,
			wantNote:  "syntax / type error",
		},
		{
			name:      "Node unhandled rejection",
			output:    "(node:12345) UnhandledPromiseRejection: This error originated either by throwing inside of an async function without a catch block...",
			exitCode:  1,
			wantScore: 1,
			wantNote:  "unhandled promise rejection / fatal error",
		},
		{
			name:      "Node ReferenceError",
			output:    "ReferenceError: foo is not defined\n    at Object.<anonymous> (/app/index.js:2:1)",
			exitCode:  1,
			wantScore: 2,
			wantNote:  "javascript syntax / type error",
		},
		{
			name:      "Generic command not found",
			output:    "bash: foo_bar_baz: command not found",
			exitCode:  127,
			wantScore: 1,
			wantNote:  "command not found / permission denied",
		},
		{
			name:      "Generic non-zero exit without specific pattern",
			output:    "some unknown error occurred",
			exitCode:  1,
			wantScore: 2,
			wantNote:  "command failed with exit code 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, note := ScoreShell(tt.output, tt.exitCode)
			if score != tt.wantScore {
				t.Errorf("ScoreShell() score = %d, want %d", score, tt.wantScore)
			}
			if tt.wantNote != "" && note != tt.wantNote {
				t.Errorf("ScoreShell() note = %q, want %q", note, tt.wantNote)
			}
		})
	}
}
