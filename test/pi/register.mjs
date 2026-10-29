// Preload for the Pi extension tests: node --import ./test/pi/register.mjs --test test/pi/background-tasks.test.ts
import { register } from "node:module";

register("./typebox-hook.mjs", import.meta.url);
