User: explain in detail how "claude" "agy" and "codex" compaction is triggered with Jev. Which hooks are used, which files are written, etc.
https://github.com/tamaratran/fast-jev-compaction

Agent: I'll start by exploring the repository structure to understand how Claude, Agy, and Codex compaction is triggered with Jev.Let me search for the main files to understand the structure:Now I have all the information needed to provide a detailed explanation of how compaction is triggered and works with Jev.

## Detailed Explanation: Compaction Triggered with Jev

This repository implements a Claude Code plugin that replaces Claude's built-in compaction summary with **Jev-guided decisions**. Here's how it works in detail:

---

### **Hooks That Trigger Compaction**

Two **function hooks** registered in `hooks/fast-jev.ts` trigger the compaction flow:

#### 1. **`session.compact` Hook** (Primary Trigger)
```typescript
on('session.compact', async ($, event, next) => {
  // Explicit compaction request (user manual `/compact` or auto-compaction)
  const { result, messages } = await compactSession(event.messages, config, fetchFn);
  
  if (reductionRatio(result) < config.minReductionRatio) {
    return next(event);  // Fallback to built-in summary if reduction too low
  }
  return { messages };  // Replace with compacted transcript
});
```

**When it fires:**
- User runs `/compact` command in Claude Code
- Auto-compaction triggers after `turn.complete` (see below)

**What happens:**
- Takes the `event.messages` (session transcript)
- Calls `compactSession()` to run Jev scoring
- If reduction ≥ `minReductionRatio` (default 25%), replaces the session messages
- Otherwise, delegates to Claude's built-in summary via `next(event)`

#### 2. **`turn.complete` Hook** (Auto-Trigger)
```typescript
on('turn.complete', async ($, event, next) => {
  const { context } = await $.session.usage();
  if ((context.percent ?? 0) < configured.compactAtPercent) return next(event);  // Default: 60%
  
  compacting = true;
  await $.session.compact();  // Triggers the session.compact hook above
});
```

**When it fires:**
- After every turn completes
- Checks if context usage ≥ `compactAtPercent` (default 60%)
- If so, calls `$.session.compact()`, which triggers the `session.compact` hook
- Guards against concurrent compactions with `compacting` flag

---

### **The Compaction Flow: What Files Are Written**

The compaction process doesn't write new files to disk—it **rebuilds the in-memory session transcript**. Here's the sequence:

#### **Stage 1: Collect Tool Calls** (`src/state.ts: collectToolCalls()`)
- Pairs every `tool_use` with its matching `tool_result` by `tool_use_id`
- Skips calls without results (nothing to drop yet)
- Marks calls as **"pinned"** if they're in:
  - The **first message** (always preserved)
  - The **newest `preserveRecentMessages`** messages (default 6)
- Returns a flat list of `ToolCall` objects with metadata

#### **Stage 2: Build Jev State** (`src/state.ts: fitState()`)
- Constructs a `CompactionState` containing:
  - `context`: Instructional preamble about compaction
  - `goal`: Last 3 user prompts (what the assistant was working on)
  - `history`: Array of `HistoryEntry` objects, one per message

- Each history entry includes:
  - `role`: "user" or "assistant"
  - `text`: Full message text (abridged only in Jev state, kept verbatim in output)
  - `tool_calls`: Tool call info **with results replaced by a one-line note** like `ok, 4213 chars (omitted)`

**Fitting Stages** (shrink iteratively until state fits `maxStateTokens` budget):

1. **Stage 1 (Full)**: Include all tool inputs fully
2. **Stage 2 (inputs ≤ 200)**: Truncate inputs to 200 chars
3. **Stage 3 (inputs ≤ 60)**: Truncate inputs to 60 chars
4. **Stage 4 (texts abridged)**: Long message texts → `[head 400 chars]…[tail 150 chars]`
5. **Stage 5 (messages collapsed)**: Old non-pinned text messages → `[… N chars omitted …]`
6. **Stage 6 (calls compacted)**: Old tool calls → single-line format: `t12 Read file_path=src/a.ts → ok 480ch`
7. **Stage 7 (messages left out)**: Drop old text-only messages entirely
8. **Stage 8 (calls merged)**: Fold runs of consecutive call-only entries into one

If it still doesn't fit: **throws error**, hook catches it and falls back to built-in summary.

#### **Stage 3: Ask Jev** (`src/compact.ts: askBatch()` + `src/request.ts`)

For each **non-pinned** `ToolCall`, Jev is asked **two `noul` (yes/no) questions**:

```typescript
export function questionsFor(call: ToolCall): JevQuestions {
  return {
    [`call_${call.id}`]: {
      type: 'noul',
      instructions: `Tool call ${call.id} (${call.tool}) should stay in the history: knowing this call was made, with its input, still matters for what the assistant does next`,
    },
    [`result_${call.id}`]: {
      type: 'noul',
      instructions: `The full output of tool call ${call.id} (${call.tool}, ${call.resultChars} chars) should stay in the history verbatim: the assistant still needs its contents and re-running the tool would not do.`,
    },
  };
}
```

**The HTTP Request** (to TypeSafe API):
```json
POST https://api.typesafe.ai/v1/systemone
Authorization: Bearer <TYPESAFE_API_KEY>
Content-Type: application/json

{
  "model": "jev-latest",
  "state": { ... entire conversation history ... },
  "questions": { "call_t1": {...}, "result_t1": {...}, "call_t2": {...}, ... }
}
```

**Batching:**
- Questions are split across multiple requests if needed
- Each request includes the **full state** (repeated)
- Batches split so state + questions ≤ `maxRequestTokens` (default 30k)
- All batches run **concurrently**, answers merged

#### **Stage 4: Decide Per Call** (`src/compact.ts: decideCall()`)

For each call, Jev returns probabilities (`noul` values, 0–1):

```typescript
export function decideCall(
  call: ToolCall,
  answer: CallAnswer,  // { keepCall, keepResult: number 0–1 }
  options: { keepThreshold: 0.5 }
): CallDecision {
  if (call.pinned) return { action: 'keep', reason: 'pinned' };
  
  if (answer.keepResult >= options.keepThreshold) {
    return { action: 'keep', reason: 'kept' };  // Keep call AND result
  }
  
  if (answer.keepCall >= options.keepThreshold) {
    return { action: 'drop_result', reason: 'result_dropped' };  // Keep call, truncate result
  }
  
  return { action: 'drop_call', reason: 'call_dropped' };  // Remove both
}
```

**Three outcomes per call:**
- **`keep`**: Both call and result stay verbatim
- **`drop_result`**: Call stays, result truncated to `truncateHeadChars` (default 300) + note
- **`drop_call`**: Call and result both removed

#### **Stage 5: Rebuild Messages** (`src/compact.ts: applyDecisions()`)

Returns the transformed transcript:
- **Unchanged messages** → returned as-is (same object reference)
- **Modified messages** → rebuilt with filtered `toolUses` and `toolResults`
- **Empty messages** (no content left) → removed entirely
- **Tool results** with `drop_result` action → truncated to head + note:
  ```
  First 300 characters of output
  [fast-jev-compaction truncated 1024 chars of this tool result; re-run the tool if needed]
  ```

#### **Stage 6: Map Back to Session** (`hooks/fast-jev.ts: toSessionMessages()`)

Maps the compacted library `Message[]` back onto Claude Code's `SessionMessage[]`:
- Preserves handle references for unchanged messages (engine recognizes them)
- Creates fresh message objects for rebuilt ones (engine applies the edits)

---

### **Configuration Written to Files**

**`.claude-plugin/plugin.json`** defines all user-configurable options:

```json
{
  "userConfig": {
    "apiKey": {...},                    // TypeSafe API key (sensitive)
    "keepThreshold": 0.5,               // Jev probability threshold
    "preserveRecentMessages": 6,        // Pinned messages from end
    "compactAtPercent": 60,             // Trigger threshold
    "minReductionRatio": 0.25,          // Fallback if too little reduction
    "maxStateTokens": 25000,            // State budget for Jev
    "maxRequestTokens": 30000,          // Request budget (state + questions)
    "truncateHeadChars": 300,           // Chars kept in truncated results
    "model": "jev-latest"               // Jev model to use
  }
}
```

**`hooks/hooks.json`** registers the hook module:
```json
{ "modules": ["./fast-jev.ts"] }
```

---

### **Summary: When and Where Compaction Happens**

| Trigger | Condition | Hook | Outcome |
|---------|-----------|------|---------|
| **Manual** | User `/compact` | `session.compact` | Jev scores & rebuilds session messages |
| **Auto** | `turn.complete` + context ≥ 60% | `turn.complete` → `session.compact` | Same |
| **Fallback** | Jev fails, ≤25% reduction, or key missing | `session.compact` | Uses Claude Code's built-in summary |

**Files modified (in memory):**
- `session.messages`: Compacted transcript replaces original
- Logging output: Decision details, reduction ratio, state size, request count

**No permanent file writes** — compaction only affects the in-memory session state, which Claude Code then persists as normal.
