#!/usr/bin/env node
// ATAP CLI helper — register agents and make signed requests
// Usage:
//   node scripts/atap-cli.js register [name]
//   node scripts/atap-cli.js get /v1/entities/<id>
//   node scripts/atap-cli.js get /v1/me --key-id <keyId> --private-key <base64>
//   node scripts/atap-cli.js health

const crypto = require("crypto");
const http = require("http");

const BASE = process.env.ATAP_URL || "http://localhost:8080";

function sign(privateKeyB64, keyId, method, path) {
  const ts = new Date().toISOString().replace(/\.\d{3}Z$/, "Z"); // RFC3339 no millis
  const payload = `${method} ${path} ${ts}`;
  const privBytes = Buffer.from(privateKeyB64, "base64");
  const sig = crypto.sign(null, Buffer.from(payload), {
    key: Buffer.concat([
      // Ed25519 PKCS8 DER prefix (16 bytes) + 32-byte seed
      Buffer.from("302e020100300506032b657004220420", "hex"),
      privBytes.subarray(0, 32),
    ]),
    format: "der",
    type: "pkcs8",
  });
  const sigB64 = sig.toString("base64url");
  return {
    authorization: `Signature keyId="${keyId}",algorithm="ed25519",headers="(request-target) x-atap-timestamp",signature="${sigB64}"`,
    timestamp: ts,
  };
}

function request(method, path, { body, headers } = {}) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, BASE);
    const opts = {
      method,
      hostname: url.hostname,
      port: url.port,
      path: url.pathname,
      headers: { ...headers },
    };
    if (body) {
      opts.headers["Content-Type"] = "application/json";
    }
    const req = http.request(opts, (res) => {
      let data = "";
      res.on("data", (c) => (data += c));
      res.on("end", () => {
        try {
          resolve({ status: res.statusCode, body: JSON.parse(data) });
        } catch {
          resolve({ status: res.statusCode, body: data });
        }
      });
    });
    req.on("error", reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

async function main() {
  const [, , cmd, ...args] = process.argv;

  if (!cmd || cmd === "help") {
    console.log(`ATAP CLI helper

Commands:
  health                                          Check server health
  register [name]                                 Register a new agent
  get <path> --key-id <id> --private-key <b64>    Signed GET request

Environment:
  ATAP_URL  Base URL (default: http://localhost:8080)

Examples:
  node scripts/atap-cli.js health
  node scripts/atap-cli.js register my-agent
  node scripts/atap-cli.js get /v1/me --key-id key_ag_abc123 --private-key <b64>`);
    return;
  }

  if (cmd === "health") {
    const res = await request("GET", "/v1/health");
    console.log(JSON.stringify(res.body, null, 2));
    return;
  }

  if (cmd === "register") {
    const name = args[0] || "test-agent";
    const res = await request("POST", "/v1/register", {
      body: { name },
    });
    console.log(`Status: ${res.status}\n`);
    console.log(JSON.stringify(res.body, null, 2));
    if (res.status === 201) {
      console.log(`\nTo make signed requests:\n`);
      console.log(
        `  node scripts/atap-cli.js get /v1/me --key-id ${res.body.key_id} --private-key ${res.body.private_key}`
      );
      console.log(
        `  node scripts/atap-cli.js get /v1/entities/${res.body.id}`
      );
    }
    return;
  }

  if (cmd === "get") {
    const path = args[0];
    if (!path) {
      console.error("Usage: get <path> [--key-id <id> --private-key <b64>]");
      process.exit(1);
    }

    const headers = {};
    const keyIdIdx = args.indexOf("--key-id");
    const privIdx = args.indexOf("--private-key");

    if (keyIdIdx >= 0 && privIdx >= 0) {
      const keyId = args[keyIdIdx + 1];
      const privKey = args[privIdx + 1];
      const { authorization, timestamp } = sign(privKey, keyId, "GET", path);
      headers["Authorization"] = authorization;
      headers["X-Atap-Timestamp"] = timestamp;
    }

    const res = await request("GET", path, { headers });
    console.log(`Status: ${res.status}\n`);
    console.log(JSON.stringify(res.body, null, 2));
    return;
  }

  console.error(`Unknown command: ${cmd}. Run with 'help' for usage.`);
  process.exit(1);
}

main().catch((e) => {
  console.error(e.message);
  process.exit(1);
});
