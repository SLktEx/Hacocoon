// Executed only inside the Env by the matching VS Code Server's Node runtime.
// Input is data on stdin. Nothing from the guest is executed on the Host.
const fs = require('fs');
const cp = require('child_process');
const crypto = require('crypto');
const path = require('path');
const C = fs.constants;
const marker = '// Hacocoon managed keys: ';

function directory(parent, name) {
  const child = `/proc/self/fd/${parent}/${name}`;
  try { fs.mkdirSync(child, {mode: 0o700}); } catch (e) { if (e.code !== 'EEXIST') throw e; }
  return fs.openSync(child, C.O_RDONLY | C.O_DIRECTORY | C.O_NOFOLLOW);
}
function readFile(parent, name) {
  let fd;
  try { fd = fs.openSync(`/proc/self/fd/${parent}/${name}`, C.O_RDONLY | C.O_NOFOLLOW | C.O_NONBLOCK); }
  catch (e) { if (e.code === 'ENOENT') return null; throw e; }
  try {
    const st = fs.fstatSync(fd);
    if (!st.isFile() || st.nlink !== 1 || st.size > 2 * 1024 * 1024) throw Error('unsafe file');
    return fs.readFileSync(fd, 'utf8');
  } finally { fs.closeSync(fd); }
}
function settings(home, desired) {
  const handles = [];
  try {
    let fd = fs.openSync(home, C.O_RDONLY | C.O_DIRECTORY | C.O_NOFOLLOW); handles.push(fd);
    for (const name of ['.vscode-server', 'data', 'Machine']) { fd = directory(fd, name); handles.push(fd); }
    const old = readFile(fd, 'settings.json');
    let body = old, keys = [];
    if (body && body.startsWith(marker)) {
      const newline = body.indexOf('\n');
      keys = JSON.parse(body.slice(marker.length, newline)); body = body.slice(newline + 1);
      if (!Array.isArray(keys) || keys.some(k => typeof k !== 'string')) throw Error('invalid ownership');
    }
    const current = body && body.trim() ? JSON.parse(body) : {};
    if (!current || typeof current !== 'object' || Array.isArray(current)) throw Error('invalid settings');
    for (const k of keys) delete current[k];
    // defineProperty treats __proto__ as a setting name, never as a prototype.
    for (const [k, v] of Object.entries(desired)) Object.defineProperty(current, k, {value: v, enumerable: true, configurable: true, writable: true});
    const next = marker + JSON.stringify(Object.keys(desired)) + '\n' + JSON.stringify(current, null, 2) + '\n';
    const temp = `/proc/self/fd/${fd}/.haco-${crypto.randomBytes(16).toString('hex')}`;
    let file;
    try {
      file = fs.openSync(temp, C.O_WRONLY | C.O_CREAT | C.O_EXCL, 0o600);
      fs.writeFileSync(file, next); fs.fsyncSync(file); fs.closeSync(file); file = undefined;
      if (readFile(fd, 'settings.json') !== old) throw Error('settings changed');
      fs.renameSync(temp, `/proc/self/fd/${fd}/settings.json`); fs.fsyncSync(fd);
    } finally {
      if (file !== undefined) fs.closeSync(file);
      try { fs.unlinkSync(temp); } catch (e) { if (e.code !== 'ENOENT') throw e; }
    }
  } finally { for (const fd of handles.reverse()) fs.closeSync(fd); }
}

function apply(input) {
  const plan = JSON.parse(input);
  if (!plan.settings || typeof plan.settings !== 'object' || Array.isArray(plan.settings) || !Array.isArray(plan.install)) throw Error('invalid plan');
  const home = require('os').homedir();
  // Serialize Hacocoon applications with an OS-owned advisory lock in the caller.
  settings(home, plan.settings);
  const server = path.dirname(process.execPath);
  const cli = path.join(server, 'out', 'server-main.js');
  function run(args) {
    const r = cp.spawnSync(process.execPath, [cli, '--server-data-dir', path.join(home, '.vscode-server'), ...args], {
      encoding: 'utf8', timeout: 120000, maxBuffer: 2 * 1024 * 1024,
      stdio: ['ignore', 'pipe', 'pipe']
    });
    if (r.error || r.status !== 0) throw Error('extension CLI failed');
    return r.stdout;
  }
  for (const e of plan.install) {
    if (!/^[a-z0-9][a-z0-9-]*\.[a-z0-9][a-z0-9-]*$/.test(e.id) || !/^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]+)?$/.test(e.version)) throw Error('invalid extension');
    const args = ['--install-extension', `${e.id}@${e.version}`, '--do-not-include-pack-dependencies', '--do-not-sync'];
    if (e.preRelease) args.push('--pre-release');
    run(args);
  }
  if (plan.install.length) {
    const actual = new Set(run(['--list-extensions', '--show-versions']).split(/\r?\n/).map(s => s.trim().toLowerCase()));
    if (plan.install.some(e => !actual.has(`${e.id}@${e.version}`.toLowerCase()))) throw Error('extension version mismatch');
  }
  process.stdout.write('haco-vscode-applied\n');
}

function lockedApply(input) {
  if (process.argv[1] === 'locked') return apply(input);
  const home = fs.openSync(require('os').homedir(), C.O_RDONLY | C.O_DIRECTORY | C.O_NOFOLLOW);
  let dir, lock;
  try {
    dir = directory(home, '.vscode-server');
    lock = fs.openSync(`/proc/self/fd/${dir}/.haco-apply.lock`, C.O_RDWR | C.O_CREAT | C.O_NOFOLLOW | C.O_NONBLOCK, 0o600);
    const st = fs.fstatSync(lock);
    if (!st.isFile() || st.nlink !== 1) throw Error('unsafe lock');
    const r = cp.spawnSync('/usr/bin/flock', ['-x', '-w', '120', '3', process.execPath, '-e', process._eval, 'locked'], {
      input, encoding: 'utf8', timeout: 14 * 60 * 1000, maxBuffer: 8192,
      stdio: ['pipe', 'pipe', 'pipe', lock]
    });
    if (r.error || r.status !== 0 || r.stdout !== 'haco-vscode-applied\n') throw Error('application failed');
    process.stdout.write(r.stdout);
  } finally {
    if (lock !== undefined) fs.closeSync(lock);
    if (dir !== undefined) fs.closeSync(dir);
    fs.closeSync(home);
  }
}

let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => { input += chunk; if (Buffer.byteLength(input) > 2 * 1024 * 1024) process.exit(1); });
process.stdin.on('end', () => {
  try { lockedApply(input); } catch (_) { process.stderr.write('VS Code configuration or extension installation failed inside the Env\n'); process.exitCode = 1; }
});
