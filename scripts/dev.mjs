#!/usr/bin/env node

import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";
import process from "node:process";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, "..");

const DEFAULT_JWT_SECRET =
  "dev_jwt_secret_9b1d3f4e5a6c7d8e9f0123456789abcdef0123456789abcdef";
const DEFAULT_VAULT_KEY =
  "9f8e7d6c5b4a392817160f0e0d0c0b0a99887766554433221100ffeeddccbbaa";

const env = { ...process.env };

if (!env.SHELLCN_AUTH_JWT_SECRET) {
  console.log(
    "[dev] Using default development JWT secret (set SHELLCN_AUTH_JWT_SECRET to override)."
  );
  env.SHELLCN_AUTH_JWT_SECRET = DEFAULT_JWT_SECRET;
}

if (!env.SHELLCN_VAULT_ENCRYPTION_KEY) {
  console.log(
    "[dev] Using default development vault key (set SHELLCN_VAULT_ENCRYPTION_KEY to override)."
  );
  env.SHELLCN_VAULT_ENCRYPTION_KEY = DEFAULT_VAULT_KEY;
}

env.SHELLCN_SERVER_PORT = env.SHELLCN_SERVER_PORT || "8000";
env.NODE_ENV = env.NODE_ENV || "development";

const processes = [
  {
    name: "server",
    command: process.platform === "win32" ? "go.exe" : "go",
    args: ["run", "./cmd/server"],
    cwd: projectRoot,
  },
  {
    name: "web",
    command: process.platform === "win32" ? "pnpm.cmd" : "pnpm",
    args: ["dev"],
    cwd: path.join(projectRoot, "web"),
  },
];

const running = new Set();
let shuttingDown = false;

function prefix(name, message) {
  return `[${name}] ${message}`;
}

function forwardStream(stream, name, destination) {
  stream.on("data", (chunk) => {
    const lines = chunk.toString().split(/\r?\n/);
    lines
      .filter((line) => line.trim().length > 0)
      .forEach((line) => {
        destination.write(`${prefix(name, line)}\n`);
      });
  });
}

function startProcess({ name, command, args, cwd }) {
  const child = spawn(command, args, {
    cwd,
    env,
    stdio: ["inherit", "pipe", "pipe"],
    windowsHide: false,
  });

  running.add(child);

  forwardStream(child.stdout, name, process.stdout);
  forwardStream(child.stderr, name, process.stderr);

  child.on("exit", (code, signal) => {
    running.delete(child);
    if (shuttingDown) {
      return;
    }

    const reason =
      typeof code === "number"
        ? `exited with code ${code}`
        : `terminated by signal ${signal || "unknown"}`;
    process.stderr.write(prefix(name, `process ${reason}`) + "\n");
    shutdown(code ?? 1);
  });

  return child;
}

function shutdown(exitCode = 0) {
  if (shuttingDown) {
    return;
  }
  shuttingDown = true;
  for (const child of running) {
    if (!child.killed) {
      child.kill("SIGINT");
    }
  }
  setTimeout(() => {
    for (const child of running) {
      if (!child.killed) {
        child.kill("SIGTERM");
      }
    }
  }, 200);
  setTimeout(() => {
    process.exit(exitCode);
  }, 200);
}

process.on("SIGINT", () => {
  process.stdout.write("\nReceived SIGINT — shutting down...\n");
  shutdown(0);
});

process.on("SIGTERM", () => {
  process.stdout.write("\nReceived SIGTERM — shutting down...\n");
  shutdown(0);
});

process.on("uncaughtException", (error) => {
  console.error(error);
  shutdown(1);
});

processes.forEach(startProcess);
