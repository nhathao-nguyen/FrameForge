import { readFileSync } from "node:fs";
import { createHash, webcrypto } from "node:crypto";

const { subtle } = webcrypto;

function option(name) {
  const index = process.argv.indexOf(name);
  if (index < 0 || index + 1 >= process.argv.length) {
    throw new Error(`missing option ${name}`);
  }
  return process.argv[index + 1];
}

function decodeOuterBase64(path) {
  return Buffer.from(readFileSync(path, "utf8").trim(), "base64").toString("utf8");
}

function decodePublicKey(path) {
  const lines = decodeOuterBase64(path).split(/\r?\n/);
  if (lines.length < 2) throw new Error("invalid Tauri public key encoding");
  const encoded = Buffer.from(lines[1].trim(), "base64");
  if (encoded.length !== 42) throw new Error("invalid Minisign public key length");
  return { algorithm: encoded.subarray(0, 2), keyId: encoded.subarray(2, 10), key: encoded.subarray(10) };
}

function decodeSignature(path) {
  const lines = decodeOuterBase64(path).split(/\r?\n/);
  if (lines.length < 4) throw new Error("invalid Tauri signature encoding");
  const encoded = Buffer.from(lines[1].trim(), "base64");
  const globalSignature = Buffer.from(lines[3].trim(), "base64");
  if (encoded.length !== 74 || globalSignature.length !== 64) {
    throw new Error("invalid Minisign signature length");
  }
  const trustedComment = lines[2];
  if (!trustedComment.startsWith("trusted comment: ")) {
    throw new Error("invalid Minisign trusted comment");
  }
  return {
    algorithm: encoded.subarray(0, 2),
    keyId: encoded.subarray(2, 10),
    signature: encoded.subarray(10),
    trustedComment: trustedComment.slice("trusted comment: ".length),
    globalSignature,
  };
}

async function verify() {
  const publicKey = decodePublicKey(option("--public-key"));
  const signature = decodeSignature(option("--signature"));
  if (!publicKey.keyId.equals(signature.keyId)) throw new Error("signature key id does not match public key");

  const fileBytes = readFileSync(option("--file"));
  let message = fileBytes;
  if (signature.algorithm[0] === 0x45 && signature.algorithm[1] === 0x44) {
    message = createHash("blake2b512").update(fileBytes).digest();
  } else if (!(signature.algorithm[0] === 0x45 && signature.algorithm[1] === 0x64)) {
    throw new Error("unsupported Minisign signature algorithm");
  }

  const key = await subtle.importKey("raw", publicKey.key, { name: "Ed25519" }, false, ["verify"]);
  const valid = await subtle.verify("Ed25519", key, signature.signature, message);
  if (!valid) throw new Error("file signature is invalid");

  const globalMessage = Buffer.concat([signature.signature, Buffer.from(signature.trustedComment)]);
  const globalValid = await subtle.verify("Ed25519", key, signature.globalSignature, globalMessage);
  if (!globalValid) throw new Error("signature trusted comment is invalid");

  console.log("Tauri Minisign signature verified");
}

verify().catch((error) => {
  console.error(`Tauri signature verification failed: ${error.message}`);
  process.exitCode = 1;
});
