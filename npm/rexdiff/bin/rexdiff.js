#!/usr/bin/env node
/**
 * Runs the rexdiff binary for this platform. npm installs it from the
 * optional dependency that matches the platform, such as
 * @segersniels/rexdiff-darwin-arm64.
 */
"use strict";

const { spawnSync } = require("node:child_process");

const platformPackage = `@segersniels/rexdiff-${process.platform}-${process.arch}`;

let binary;
try {
  binary = require.resolve(`${platformPackage}/bin/rexdiff`);
} catch {
  console.error(
    `rexdiff: no binary for ${process.platform}-${process.arch}.\n` +
      `Reinstall without --no-optional, or download a release from https://github.com/segersniels/rexdiff/releases.`,
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`rexdiff: ${result.error.message}`);
  process.exit(1);
}

// Exit the same way as the binary, so callers see its signal or status.
if (result.signal) {
  process.kill(process.pid, result.signal);
}

process.exit(result.status ?? 1);
