export const register = (on: any) => {
  on('session.compact', async ($: any) => {
    const global = globalThis as any;
    const processObject = global.process;
    const requireFunction = global.require;
    const report: any = {
      hook: 'session.compact bridge B',
      process: typeof processObject,
      processKeys: processObject == null ? [] : Object.keys(processObject),
      getBuiltinModule: typeof processObject?.getBuiltinModule,
      require: typeof requireFunction,
    };

    try {
      let childProcess: any;
      if (typeof processObject?.getBuiltinModule === 'function') {
        childProcess = processObject.getBuiltinModule('node:child_process');
        report.route = 'process.getBuiltinModule';
      } else if (typeof requireFunction === 'function') {
        childProcess = requireFunction('node:child_process');
        report.route = 'globalThis.require';
      } else {
        report.route = 'not available';
      }
      if (childProcess) {
        report.version = childProcess.execFileSync('harnez', ['--version'], { encoding: 'utf8' });
      }
    } catch (error) {
      report.error = String(error);
    }

    $.ui.log(JSON.stringify(report));
    return { messages: [{ role: 'assistant', text: 'HARNEZ_646_BRIDGE_B', toolUses: [] }] };
  });
};
