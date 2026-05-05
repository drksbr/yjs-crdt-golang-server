import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";

const frontendDir = process.cwd();
const repoRoot = path.resolve(frontendDir, "../..");
const dataDir = path.join(repoRoot, "storage", "data");
const nextBinary = path.join(frontendDir, "node_modules", ".bin", "next");

function parseEnvFile(filePath) {
  if (!fs.existsSync(filePath)) {
    return {};
  }

  const source = fs.readFileSync(filePath, "utf8");
  const entries = {};

  for (const rawLine of source.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;

    const separatorIndex = line.indexOf("=");
    if (separatorIndex === -1) continue;

    const key = line.slice(0, separatorIndex).trim();
    let value = line.slice(separatorIndex + 1).trim();

    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }

    entries[key] = value;
  }

  return entries;
}

function portFromAddress(address) {
  const value = String(address || "").trim();
  const match = value.match(/:(\d+)$/) || value.match(/^(\d+)$/);
  return match ? match[1] : "8080";
}

const envFromDotEnv = parseEnvFile(path.join(frontendDir, ".env"));
const envFromDotEnvLocal = parseEnvFile(path.join(frontendDir, ".env.local"));
const mergedFileEnv = { ...envFromDotEnv, ...envFromDotEnvLocal };

const nextPort = process.env.PORT || mergedFileEnv.PORT || "3000";
const backendAddress =
  process.env.DONTPAD_ADDR || mergedFileEnv.DONTPAD_ADDR || ":8080";
const backendPort = portFromAddress(backendAddress);
const backendURL =
  process.env.DONTPAD_BACKEND_URL ||
  mergedFileEnv.DONTPAD_BACKEND_URL ||
  `http://127.0.0.1:${backendPort}`;
const jwtSecret =
  process.env.JWT_SECRET ||
  mergedFileEnv.JWT_SECRET ||
  "local-dev-only-jwt-secret-change-me";

fs.mkdirSync(dataDir, { recursive: true });

const baseEnv = {
  ...process.env,
  ...mergedFileEnv,
  NODE_ENV: "development",
  JWT_SECRET: jwtSecret,
};

const backendEnv = {
  ...baseEnv,
  DATABASE_URL:
    process.env.DATABASE_URL ||
    mergedFileEnv.DATABASE_URL ||
    "postgres://postgres@127.0.0.1:5432/dontpadbr3",
  DONTPAD_ADDR: backendAddress,
  DONTPAD_SCHEMA:
    process.env.DONTPAD_SCHEMA || mergedFileEnv.DONTPAD_SCHEMA || "dontpadbr3",
  DONTPAD_NAMESPACE:
    process.env.DONTPAD_NAMESPACE ||
    mergedFileEnv.DONTPAD_NAMESPACE ||
    "dontpadbr3",
  DONTPAD_DATA_DIR: dataDir,
  DONTPAD_ALLOWED_ORIGINS:
    process.env.DONTPAD_ALLOWED_ORIGINS ||
    mergedFileEnv.DONTPAD_ALLOWED_ORIGINS ||
    `http://127.0.0.1:${nextPort},http://localhost:${nextPort}`,
};

const nextEnv = {
  ...baseEnv,
  PORT: nextPort,
  DONTPAD_BACKEND_URL: backendURL,
};

console.log("");
console.log("Local dev mode");
console.log(`  Next.js : http://127.0.0.1:${nextPort}`);
console.log(`  Backend : ${backendURL}`);
console.log(`  Data    : ${dataDir}`);
console.log("");

const children = [];
let shuttingDown = false;

function spawnService(label, command, args, env, cwd) {
  const child = spawn(command, args, {
    cwd,
    env,
    stdio: "inherit",
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      console.log(`[${label}] stopped by signal ${signal}`);
      return;
    }

    if (code && code !== 0) {
      console.error(`[${label}] exited with code ${code}`);
      shutdown(code);
    }
  });

  children.push(child);
  return child;
}

function shutdown(exitCode = 0) {
  if (shuttingDown) return;
  shuttingDown = true;

  for (const child of children) {
    if (!child.killed) {
      child.kill("SIGTERM");
    }
  }

  setTimeout(() => {
    for (const child of children) {
      if (!child.killed) {
        child.kill("SIGKILL");
      }
    }
    process.exit(exitCode);
  }, 1500).unref();
}

process.on("SIGINT", () => shutdown(0));
process.on("SIGTERM", () => shutdown(0));

spawnService("backend", "go", ["run", "./apps/backend"], backendEnv, repoRoot);
spawnService(
  "next",
  nextBinary,
  ["dev", "--webpack", "--port", nextPort],
  nextEnv,
  frontendDir,
);
