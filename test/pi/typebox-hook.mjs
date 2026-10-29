// Resolve hook: maps the bare "typebox" import to the test stub, so tests never load the home Pi install.
const STUB = new URL("./typebox-stub.mjs", import.meta.url).href;

export async function resolve(specifier, context, nextResolve) {
  if (specifier === "typebox") return { url: STUB, shortCircuit: true };
  return nextResolve(specifier, context);
}
