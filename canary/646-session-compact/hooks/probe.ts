export const register = (on: any) => {
  on('session.compact', async ($: any, event: any) => {
    const messagesJSON = JSON.stringify(event.messages);
    const report: any = {
      hook: 'session.compact bridge A pre-work',
      dollarKeyListing: 'unavailable: function-hook grammar rejects reading $ nouns as values',
      eventKeys: Object.keys(event),
      eventFields: Object.fromEntries(Object.entries(event).map(([key, value]) => [
        key,
        value === null ? 'null' : typeof value === 'object'
          ? Array.isArray(value) ? `array(${value.length})` : `object(${Object.keys(value).join(',')})`
          : `${typeof value}:${String(value).slice(0, 100)}`,
      ])),
      messageCount: event.messages.length,
      messagesJSONBytes: new TextEncoder().encode(messagesJSON).length,
      messageShapes: event.messages.map((message: any) => ({
        keys: Object.keys(message),
        role: message.role,
        textType: typeof message.text,
        handle: message.handle == null ? null : typeof message.handle === 'object'
          ? Object.fromEntries(Object.entries(message.handle).map(([key, value]) => [
            key,
            value === null ? 'null' : typeof value === 'object'
              ? `object(${Object.keys(value).join(',')})`
              : `${typeof value}:${String(value).slice(0, 100)}`,
          ]))
          : `${typeof message.handle}:${String(message.handle).slice(0, 100)}`,
      })),
      fetch: typeof globalThis.fetch,
      process: typeof (globalThis as any).process,
      Bun: typeof (globalThis as any).Bun,
      Deno: typeof (globalThis as any).Deno,
      require: typeof (globalThis as any).require,
    };

    try {
      const small = await $.process.run(['harnez', 'read', '-'], { stdin: 'HARNEZ_STDIN_PROBE' });
      report.smallStdin = {
        exitCode: small.exitCode,
        stdout: small.stdout,
        stderr: small.stderr,
        stdoutBytes: new TextEncoder().encode(small.stdout ?? '').length,
      };
      if (small.stdout?.trimEnd() === 'HARNEZ_STDIN_PROBE') {
        const largeInput = 'x'.repeat(1024 * 1024);
        const large = await $.process.run(['harnez', 'read', '-'], { stdin: largeInput });
        report.largeStdin = {
          exitCode: large.exitCode,
          stdoutBytes: new TextEncoder().encode(large.stdout ?? '').length,
          stdoutTruncated: large.isStdoutTruncated,
          exact: large.stdout?.trimEnd() === largeInput,
        };
      }
    } catch (error) {
      report.stdinError = String(error);
    }

    $.ui.log(JSON.stringify(report));
    return { messages: event.messages };
  });
};
