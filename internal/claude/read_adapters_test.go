package claude

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise generated adapters, including their real child-process JSON boundary.
// This is a contract canary, not a claim that a live agent loaded the plugin.
func TestReadAdaptersRuntimeCanary(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	dir := t.TempDir()
	files := map[string]string{
		"pi.ts":       piDistillAdapter,
		"opencode.ts": openCodeDistillAdapter,
		"harnez": `#!/usr/bin/env node
let input="";
process.stdin.on("data", data => input+=data);
process.stdin.on("end", () => {
  const request=JSON.parse(input);
  if (process.argv[2]!=="distill" || process.argv[3]!=="read" || request.text!=="original") process.exit(2);
  process.stdout.write(JSON.stringify({text:"bounded",truncated:true}));
});
`,
		"canary.mjs": `import assert from "node:assert/strict";
import pi from "./pi.ts";
import { HarnezDistillPlugin } from "./opencode.ts";
const handlers={};
pi({on:(name,handler)=>handlers[name]=handler});
const event={toolName:"read",input:{path:"sample.go",offset:2},content:[{type:"text",text:"original"}],isError:false};
delete process.env.HARNEZ_READ_AUTOPIPE;
assert.equal(await handlers.tool_result(event),undefined);
process.env.HARNEZ_READ_AUTOPIPE="1";
assert.equal((await handlers.tool_result(event)).content[0].text,"bounded");
assert.equal(await handlers.tool_result({...event,isError:true}),undefined);
assert.equal(await handlers.tool_result({...event,toolName:"bash"}),undefined);
const plugin=await HarnezDistillPlugin({});
const output={output:"original",title:"source",metadata:{sentinel:1}};
await plugin["tool.execute.after"]({tool:"read",args:{filePath:"sample.go"}},output);
assert.equal(output.output,"bounded");
assert.equal(output.metadata.sentinel,1);
assert.equal(output.title,"source");
console.log("read adapter contract canary passed");
`,
	}
	for name, content := range files {
		mode := os.FileMode(0600)
		if name == "harnez" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(node, "--experimental-strip-types", filepath.Join(dir, "canary.mjs"))
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated adapter runtime canary: %v\n%s", err, output)
	}
}
