import { spawn } from "node:child_process";

// Fixed, INSECURE dev master key (exactly 32 bytes) so encrypted secrets in the
// local dev DB stay decryptable across restarts — otherwise --dev mints a random
// ephemeral key each run. Never use this anywhere but local development.
const DEV_MASTER_KEY = "shellcn-dev-insecure-master-key!";

const children = [
  spawn("go", ["run", "./cmd/server", "--dev"], {
    stdio: "inherit",
    env: {
      ...process.env,
      SHELLCN_MASTER_KEY: process.env.SHELLCN_MASTER_KEY || DEV_MASTER_KEY,
    },
  }),
  spawn("pnpm", ["dev"], { cwd: "web", stdio: "inherit" }),
];

let shuttingDown = false;
function shutdown(code) {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const child of children) child.kill("SIGTERM");
  process.exit(code ?? 0);
}

process.on("SIGINT", () => shutdown(0));
process.on("SIGTERM", () => shutdown(0));
for (const child of children) {
  child.on("exit", (code) => shutdown(code));
}
