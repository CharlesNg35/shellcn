import { spawn } from "node:child_process";

const children = [
  spawn("go", ["run", "./cmd/server", "--dev"], { stdio: "inherit" }),
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
