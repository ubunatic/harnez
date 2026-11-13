// Self-contained Pi extension enforcing Harnez's per-turn tool-call limit.
// Kept out of .pi/extensions/ so Pi does not autoload it on plain runs;
// Harnez writes it to a content-hashed path and passes `pi -e <path>` only
// when a tool-call limit > 0 is active.

type ToolCallResult = { block?: boolean; reason?: string; terminate?: boolean } | undefined;

type PiExtensionAPI = {
	on(event: "tool_call", handler: () => ToolCallResult | Promise<ToolCallResult>): void;
	on(event: "turn_start" | "turn_end", handler: () => void): void;
};

function parsePositiveIntEnv(raw: string | undefined, name: string): number {
	if (raw === undefined || raw.trim() === "") {
		throw new Error(`${name} must be set to a positive integer`);
	}
	if (!/^[1-9]\d*$/.test(raw.trim())) {
		throw new Error(`${name} must be a positive integer, got ${JSON.stringify(raw)}`);
	}
	const n = Number(raw.trim());
	if (!Number.isSafeInteger(n) || n <= 0) {
		throw new Error(`${name} out of range: ${JSON.stringify(raw)}`);
	}
	return n;
}

export function installToolCallLimit(
	pi: PiExtensionAPI,
	env: Record<string, string | undefined> = process.env,
): void {
	const limit = parsePositiveIntEnv(env.HARNEZ_TOOL_CALL_LIMIT, "HARNEZ_TOOL_CALL_LIMIT");
	const prompt = env.HARNEZ_TOOL_CALL_LIMIT_PROMPT?.trim() ?? "";
	if (prompt === "") {
		throw new Error("HARNEZ_TOOL_CALL_LIMIT_PROMPT must be non-empty");
	}

	let calls = 0;
	let currentTurn = 0;
	let crossingTurn = -1;
	let pastCrossingBatch = false;

	pi.on("turn_start", () => {
		currentTurn++;
	});

	pi.on("turn_end", () => {
		if (crossingTurn !== -1) {
			pastCrossingBatch = true;
		}
	});

	pi.on("tool_call", () => {
		calls++;
		if (calls <= limit) {
			return undefined;
		}
		// Calls over the limit in the batch that crosses it get the synthesis
		// reason without terminate so the model can still answer once. Any tool
		// call in a later response terminates the loop after the batch.
		if (crossingTurn === -1) {
			crossingTurn = currentTurn;
			return { block: true, reason: prompt, terminate: false };
		}
		if (!pastCrossingBatch && currentTurn === crossingTurn) {
			return { block: true, reason: prompt, terminate: false };
		}
		return { block: true, reason: prompt, terminate: true };
	});
}

export default function toolCallLimitExtension(pi: PiExtensionAPI): void {
	installToolCallLimit(pi);
}
