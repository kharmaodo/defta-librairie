// Linux/WSL test server: disposable database, no application .env, no existing server reuse.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const {spawnSync, spawn} = require('node:child_process');
const repo = path.resolve(__dirname, '..');
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'defta-browser-'));
let server;
function clean() { fs.rmSync(temporary, {recursive:true, force:true}); }
const env = {...process.env,
  DB_PATH: path.join(temporary,'test.db'), PORT:'18080',
  JWT_SECRET:crypto.randomBytes(48).toString('hex'), JWT_ISSUER:'defta-browser',
  JWT_AUDIENCE:'defta-browser', JWT_ACCESS_TTL_SECONDS:'900', JWT_REFRESH_TTL_SECONDS:'3600',
  AUTH_COOKIE_SECURE:'false', AUTH_RATE_LIMIT_REQUESTS:'1000', AUTH_RATE_LIMIT_WINDOW_SECONDS:'60',
  DEFTA_ROOT_USERNAME:'browser-root', DEFTA_ROOT_PASSWORD:'Browser-Root-Only-2026!', DEFTA_ROOT_EMAIL:'root@example.test'
};
function run(args, cwd) {
  const result = spawnSync(args[0], args.slice(1), {cwd, env, stdio:'inherit'});
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${args[0]} failed (${result.status})`);
}
try {
  const binary = path.join(temporary,'server');
  run(['go','build','-tags','fts5','-o',binary,'./cmd'],repo);
  for (const name of ['templates','static']) fs.symlinkSync(path.join(repo,name),path.join(temporary,name),'dir');
  run([binary,'bootstrap-admin'],temporary);
  server = spawn(binary,[],{cwd:temporary,env,stdio:'inherit'});
  server.on('error',error=>{console.error(error.message);clean();process.exitCode=1;});
  server.on('exit',code=>{clean();process.exitCode=code || 0;});
  for (const signal of ['SIGTERM','SIGINT']) process.on(signal,()=>{server.kill(signal);});
} catch(error) { clean(); console.error(error.message); process.exitCode=1; }
