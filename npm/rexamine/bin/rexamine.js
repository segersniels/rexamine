#!/usr/bin/env node
/**
 * Runs the rexamine binary for this platform. npm installs it from the
 * optional dependency that matches the platform, such as
 * @segersniels/rexamine-darwin-arm64.
 */
"use strict";

const { spawnSync } = require("node:child_process");

const platformPackage = `@segersniels/rexamine-${process.platform}-${process.arch}`;

let binary;
try {
  binary = require.resolve(`${platformPackage}/bin/rexamine`);
} catch {
  console.error(
    `rexamine: no binary for ${process.platform}-${process.arch}.\n` +
      `Reinstall without --no-optional, or download a release from https://github.com/segersniels/rexamine/releases.`,
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`rexamine: ${result.error.message}`);
  process.exit(1);
}

// Exit the same way as the binary, so callers see its signal or status.
if (result.signal) {
  process.kill(process.pid, result.signal);
}

process.exit(result.status ?? 1);
