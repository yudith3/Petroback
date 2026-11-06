const fs = require("fs");
const path = require("path");
const { bruToJsonV2, bruToEnvJsonV2, collectionBruToJson } = require("@usebruno/lang");

const SENSITIVE = ["TOKEN", "PASSWORD", "EMAIL"];
const errors = [];

function walk(dir) {
  for (const name of fs.readdirSync(dir)) {
    if ([".git", "node_modules", "scripts"].includes(name)) continue;
    const file = path.join(dir, name);
    if (fs.statSync(file).isDirectory()) walk(file);
    else if (name.endsWith(".bru")) check(file, name, dir);
  }
}

function check(file, name, dir) {
  const text = fs.readFileSync(file, "utf8");
  if (/^(<{7}|={7}|>{7})/m.test(text)) {
    errors.push(`${file}: masih ada tanda merge conflict`);
    return;
  }
  try {
    if (path.basename(dir) === "environments") {
      const env = bruToEnvJsonV2(text);
      for (const v of env.variables) {
        if (SENSITIVE.includes(v.name) && !v.secret && v.value) {
          errors.push(`${file}: ${v.name} harus secret dan tidak boleh berisi nilai`);
        }
      }
    } else if (name === "folder.bru" || name === "collection.bru") {
      collectionBruToJson(text);
    } else {
      const req = bruToJsonV2(text);
      if (req.auth?.bearer?.token && !req.auth.bearer.token.startsWith("{{")) {
        errors.push(`${file}: token bearer ditulis langsung, pakai {{TOKEN}} atau auth: inherit`);
      }
    }
  } catch (e) {
    errors.push(`${file}: format .bru tidak valid (${e.message.split("\n")[0]})`);
  }
}

walk(process.cwd());
if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}
console.log("Semua file .bru valid");
