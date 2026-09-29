export const register = (on: any) => {
  on('session.compact', async ($: any, event: any) => {
    const report: any = {
      hook: 'session.compact bridge A',
      dollarKeyListing: 'unavailable: function-hook grammar rejects reading $ nouns as values',
      fetch: typeof globalThis.fetch,
      process: typeof (globalThis as any).process,
      Bun: typeof (globalThis as any).Bun,
      Deno: typeof (globalThis as any).Deno,
      require: typeof (globalThis as any).require,
    };

    try {
      report.processRunResult = await $.process.run(['harnez', '--version']);
    } catch (error) {
      report.processRunError = String(error);
    }

    try {
      const response = await globalThis.fetch('http://127.0.0.1:18764/');
      report.fetchResult = { status: response.status, body: (await response.text()).slice(0, 120) };
    } catch (error) {
      report.fetchError = String(error);
    }

    $.ui.log(JSON.stringify(report));
    return { messages: [{ role: 'assistant', text: 'HARNEZ_646_BRIDGE_A', toolUses: [] }] };
  });
};
