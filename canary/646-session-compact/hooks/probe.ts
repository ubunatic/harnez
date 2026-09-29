const describe = (value: any) => {
  if (value == null) return String(value);
  return JSON.stringify({
    type: typeof value,
    keys: typeof value === 'object' || typeof value === 'function'
      ? Object.keys(value)
      : [],
  });
};

export const register = (on: any) => {
  on('session.compact', async ($: any, event: any) => {
    const report: any = {
      hook: 'session.compact bridge probe',
      root: describe($),
      session: describe($.session),
      ui: describe($.ui),
      fetch: typeof globalThis.fetch,
      process: typeof (globalThis as any).process,
      Bun: typeof (globalThis as any).Bun,
      Deno: typeof (globalThis as any).Deno,
      require: typeof (globalThis as any).require,
    };

    for (const name of ['exec', 'shell']) {
      const route = $[name] ?? $.session?.[name] ?? $.ui?.[name];
      report[`${name}Route`] = typeof route;
      if (typeof route === 'function') {
        try {
          report[`${name}Result`] = await route('harnez --version');
        } catch (error) {
          report[`${name}Error`] = String(error);
        }
      }
    }

    try {
      const response = await globalThis.fetch('http://127.0.0.1:18764/');
      report.fetchResult = { status: response.status, body: (await response.text()).slice(0, 120) };
    } catch (error) {
      report.fetchError = String(error);
    }

    try {
      const childProcess = await import('node:child_process');
      report.childProcessImport = 'succeeded';
      if (typeof childProcess.execFileSync === 'function') {
        report.childProcessVersion = childProcess.execFileSync('harnez', ['--version'], { encoding: 'utf8' });
      }
    } catch (error) {
      report.childProcessImport = `failed: ${String(error)}`;
    }

    $.ui.log(JSON.stringify(report));
    return { messages: event.messages };
  });
};
