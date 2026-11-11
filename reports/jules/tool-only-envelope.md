## Verdict

works: The lenient approach is more robust because it correctly extracts the JSON envelope even when the model generates extraneous prose before or after the JSON.

## Approach

1. **Strict JSON parser**: Tried it; parses only pure JSON inputs, fails on mixed prose+JSON, which is less ideal for local models that sometimes "babble".
2. **Lenient JSON parser**: Tried it; finds the first `{` and iterates backwards from the end to find a matching `}` that yields a valid JSON object. This successfully handled mixed text/JSON replies while still rejecting malformed JSON.

## Tests

```
cd internal/subagent
go test -v -run TestParseEnvelope
```

Result:
```
=== RUN   TestParseEnvelope
=== RUN   TestParseEnvelope/valid_tool
=== RUN   TestParseEnvelope/valid_final
=== RUN   TestParseEnvelope/malformed_json
=== RUN   TestParseEnvelope/mixed_prose_and_json
=== RUN   TestParseEnvelope/missing_fields
--- PASS: TestParseEnvelope (0.00s)
    --- PASS: TestParseEnvelope/valid_tool (0.00s)
    --- PASS: TestParseEnvelope/valid_final (0.00s)
    --- PASS: TestParseEnvelope/malformed_json (0.00s)
    --- PASS: TestParseEnvelope/mixed_prose_and_json (0.00s)
    --- PASS: TestParseEnvelope/missing_fields (0.00s)
PASS
ok  	ubunatic.com/harnez/internal/subagent	0.027s
```

## Risks

The lenient parser approach may incorrectly extract a valid but unintended JSON object if the model produces multiple JSON blocks or includes `{` and `}` in prose before the actual tool call. Furthermore, iterating backwards through all closing braces can be slightly inefficient on very large texts.
