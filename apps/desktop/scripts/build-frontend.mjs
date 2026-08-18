import { cpSync, existsSync, mkdirSync, rmSync, symlinkSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const desktopRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const webRoot = resolve(desktopRoot, "../web");
const outputRoot = resolve(desktopRoot, "dist");
const buildRoot = resolve(desktopRoot, ".web-build");
const nextCli = resolve(webRoot, "node_modules/next/dist/bin/next");
const buildEnvironment = { ...process.env, NH_MEDIA_NEXT_DIST_DIR: ".next" };

function copyRequired(relativePath) {
  const source = resolve(webRoot, relativePath);
  if (!existsSync(source)) throw new Error(`Required web build input is missing: ${source}`);
  cpSync(source, resolve(buildRoot, relativePath), { recursive: true });
}

function copyOptional(relativePath) {
  const source = resolve(webRoot, relativePath);
  if (existsSync(source)) cpSync(source, resolve(buildRoot, relativePath), { recursive: true });
}

if (!existsSync(nextCli)) throw new Error(`Next.js build entrypoint is missing: ${nextCli}`);
rmSync(buildRoot, { recursive: true, force: true });
try {
  mkdirSync(buildRoot, { recursive: true });
  for (const input of ["src", "package.json", "tsconfig.json", "next.config.mjs", "next-env.d.ts"]) {
    copyRequired(input);
  }
  copyOptional("public");
  symlinkSync(resolve(webRoot, "node_modules"), resolve(buildRoot, "node_modules"), "junction");
  execFileSync(process.execPath, [nextCli, "build"], {
    cwd: buildRoot,
    env: buildEnvironment,
    stdio: "inherit",
  });
  rmSync(outputRoot, { recursive: true, force: true });
  mkdirSync(outputRoot, { recursive: true });
  cpSync(resolve(buildRoot, "out"), outputRoot, { recursive: true });
} finally {
  rmSync(buildRoot, { recursive: true, force: true });
}
