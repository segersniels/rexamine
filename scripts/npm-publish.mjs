/**
 * Publishes rexdiff to npm from the binaries that GoReleaser built.
 *
 * It publishes one package per platform with the binary in it, such as
 * @segersniels/rexdiff-darwin-arm64, and then the main package
 * @segersniels/rexdiff, which lists them as optional dependencies. npm then
 * installs only the binary that matches the user's platform.
 *
 * Usage: node scripts/npm-publish.mjs <version> [--dry-run]
 *
 * Run it after `goreleaser release`, which writes dist/artifacts.json.
 * Versions that are already on npm are skipped, so a failed run can be repeated.
 */

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const SCOPE = "@segersniels";
const NAME = "rexdiff";
const OUT_DIR = "dist/npm";

/** Maps Go's architecture names to the names npm uses in "cpu". */
const NPM_CPU = { amd64: "x64", arm64: "arm64" };

const [versionArg, ...flags] = process.argv.slice(2);
if (!versionArg) {
  console.error("Usage: node scripts/npm-publish.mjs <version> [--dry-run]");
  process.exit(2);
}

const version = versionArg.replace(/^v/, "");
const dryRun = flags.includes("--dry-run");

const main = JSON.parse(fs.readFileSync("npm/rexdiff/package.json", "utf8"));

const binaries = JSON.parse(fs.readFileSync("dist/artifacts.json", "utf8")).filter(
  (artifact) => artifact.type === "Binary" && artifact.name === NAME,
);

if (binaries.length === 0) {
  console.error("No rexdiff binaries in dist/artifacts.json. Run goreleaser first.");
  process.exit(1);
}

fs.rmSync(OUT_DIR, { recursive: true, force: true });

const platformPackages = {};
for (const binary of binaries) {
  const os = binary.goos;
  const cpu = NPM_CPU[binary.goarch];
  if (!cpu) {
    throw new Error(`No npm cpu name for GOARCH ${binary.goarch}`);
  }

  const name = `${SCOPE}/${NAME}-${os}-${cpu}`;
  const dir = path.join(OUT_DIR, `${NAME}-${os}-${cpu}`);
  fs.mkdirSync(path.join(dir, "bin"), { recursive: true });
  fs.copyFileSync(binary.path, path.join(dir, "bin", NAME));
  fs.chmodSync(path.join(dir, "bin", NAME), 0o755);
  writeJSON(path.join(dir, "package.json"), {
    name,
    version,
    description: `The ${os}-${cpu} binary of ${main.name}.`,
    os: [os],
    cpu: [cpu],
    files: ["bin"],
    preferUnplugged: true,
    repository: main.repository,
    license: main.license,
  });

  platformPackages[name] = version;
  publish(name, dir);
}

// The main package is published last, so it never points to a missing binary.
const mainDir = path.join(OUT_DIR, NAME);
fs.cpSync("npm/rexdiff", mainDir, { recursive: true });
fs.copyFileSync("README.md", path.join(mainDir, "README.md"));
fs.copyFileSync("LICENSE", path.join(mainDir, "LICENSE"));
writeJSON(path.join(mainDir, "package.json"), { ...main, version, optionalDependencies: platformPackages });
publish(main.name, mainDir);

/** Writes a value as formatted JSON. */
function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

/** Publishes the package in dir, unless this version is already on npm. */
function publish(name, dir) {
  if (isPublished(name)) {
    console.log(`${name}@${version} is already on npm, skipped.`);

    return;
  }

  const args = ["publish", "--access", "public"];
  if (dryRun) {
    args.push("--dry-run");
  }

  execFileSync("npm", args, { cwd: dir, stdio: "inherit" });
}

/** Checks whether this version of the package is on npm. */
function isPublished(name) {
  try {
    execFileSync("npm", ["view", `${name}@${version}`, "version"], { stdio: "pipe" });

    return true;
  } catch {
    return false;
  }
}
