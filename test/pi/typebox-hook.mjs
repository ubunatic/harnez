// Resolve hook: maps the bare "typebox" import to the test stub, so tests never load the home Pi install.
const STUB = new URL("./typebox-stub.mjs", import.meta.url).href;
const EXTENSION = new URL("../../.pi/extensions/background-tasks.ts", import.meta.url).href;
// background-claim.test.ts adds this query when it imports a mirrored copy directly. Copies under the real Pi loader carry no query.
const MIRROR_QUERY = "?pi-tui-stub";

export async function resolve(specifier, context, nextResolve) {
  // Only directly imported extension copies need a stub. Pi and its real loader must resolve their own package.
  if (specifier === "@earendil-works/pi-tui" && (context.parentURL === EXTENSION || context.parentURL?.endsWith(MIRROR_QUERY))) return { url: new URL("./width-stub.mjs", import.meta.url).href, shortCircuit: true };
  if (specifier === "typebox") return { url: STUB, shortCircuit: true };
  return nextResolve(specifier, context);
}
