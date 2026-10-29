// Test-only stand-in for typebox: the extension only builds parameter schemas, which the tests never validate.
export const Type = new Proxy(
  {},
  {
    get: (_target, kind) => (...args) => ({ kind: String(kind), args }),
  },
);
