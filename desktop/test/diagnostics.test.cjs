const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),os=require('node:os');
const {createDiagnostics}=require('../diagnostics.cjs');
function directory(t){const root=fs.mkdtempSync(path.join(os.tmpdir(),'cxpalm-diagnostics-'));t.after(()=>fs.rmSync(root,{recursive:true,force:true}));return root;}
function records(root){return fs.readFileSync(path.join(root,'lifecycle.jsonl'),'utf8').trim().split('\n').map(JSON.parse);}
test('lifecycle records unfinished runs, graceful exits and never labels a crash clean',t=>{
 const root=directory(t),first=createDiagnostics(root,{version:'0.0.24'});
 first.record('config-detected',{configured:true});
 const second=createDiagnostics(root,{version:'0.0.24'});
 assert.equal(records(root).filter(row=>row.event==='previous-session-unfinished').length,1);
 second.finish(0,true);second.finish(0,true);
 assert.equal(records(root).filter(row=>row.event==='process-exit').length,1);
 const third=createDiagnostics(root);assert.equal(records(root).filter(row=>row.event==='previous-session-unfinished').length,1);
 third.finish(9,false);createDiagnostics(root);
 assert.equal(records(root).filter(row=>row.event==='previous-session-unfinished').length,2);
});
test('diagnostics use a data allowlist, never log secrets, error messages or raw stacks',t=>{
 const root=directory(t),log=createDiagnostics(root,{version:'private-version-text'}),secret='fixture-sensitive-value';
 log.record('host-exit',{code:9,signal:'SIGTERM',password:secret,configPath:secret,reason:secret,type:secret,method:secret,version:secret});
 log.record(secret,{success:true});
 const error=new TypeError(secret);error.stack=`TypeError: ${secret}\n    at work (C:/private/${secret}/main.cjs:23:4)\n    at other (${secret}:1:2)`;
 log.exception(error,'uncaughtException');log.record('agent-action',{method:'stop',success:false,args:{password:secret}});
 const text=fs.readFileSync(path.join(root,'lifecycle.jsonl'),'utf8');assert.ok(!text.includes(secret));assert.ok(!text.includes('C:/private'));
 const rows=records(root);assert.equal(rows.length,4);assert.equal(rows[1].code,9);assert.equal(rows[1].signal,'SIGTERM');
 assert.deepEqual(rows[2].locations,['main.cjs:23:4']);assert.equal(rows[2].errorType,'TypeError');
 assert.equal(rows[3].method,'stop');assert.equal(rows[3].success,false);
});
test('rotation is bounded and unavailable disk never interrupts runtime operations',t=>{
 const root=directory(t),log=createDiagnostics(root,{maxBytes:700});
 for(let i=0;i<80;i++)log.record('host-exit',{code:i});
 for(const file of fs.readdirSync(root).filter(f=>f.startsWith('lifecycle')))assert.ok(fs.statSync(path.join(root,file)).size<=700);
 assert.equal(fs.readdirSync(root).filter(f=>f.startsWith('lifecycle')).length,3);
 const blocked=path.join(root,'not-a-directory');fs.writeFileSync(blocked,'fixture');
 assert.doesNotThrow(()=>{const broken=createDiagnostics(blocked);broken.record('host-connected');broken.finish(0,true);});
});
test('write failures are redacted, visible and recover when the disk becomes writable',context=>{
 const root=directory(context),append=fs.appendFileSync,secret='fixture-private-path';let blocked=true;
 context.mock.method(fs,'appendFileSync',(...args)=>{
  if(blocked){const error=new Error(secret);error.code='EPERM';throw error;}
  return append(...args);
 });
 const log=createDiagnostics(root);
 assert.deepEqual(log.status(),{available:false,failures:{log:'EPERM'}});
 assert.equal(log.record('heartbeat'),false);assert.ok(!JSON.stringify(log.status()).includes(secret));
 blocked=false;assert.equal(log.record('heartbeat'),true);
 assert.deepEqual(log.status(),{available:true,failures:{}});
});
test('failed session markers retry rather than silently retaining an older session',context=>{
 const root=directory(context),rename=fs.renameSync;let blocked=true;
 context.mock.method(fs,'renameSync',(...args)=>{
  if(blocked&&args[1]===path.join(root,'session.json')){const error=new Error('private marker error');error.code='EACCES';throw error;}
  return rename(...args);
 });
 const log=createDiagnostics(root);
 assert.deepEqual(log.status(),{available:false,failures:{marker:'EACCES'}});
 assert.equal(fs.existsSync(path.join(root,'session.json')),false);
 blocked=false;log.record('heartbeat');
 assert.deepEqual(log.status(),{available:true,failures:{}});
 assert.equal(JSON.parse(fs.readFileSync(path.join(root,'session.json'),'utf8')).clean,false);
});
test('session queries remain distinct from actual session end and only fixed reasons reach disk',context=>{
 const root=directory(context),log=createDiagnostics(root);
 log.record('system-session-query',{sessionReasons:['close-app','secret','close-app']});
 log.record('system-session-end',{sessionReasons:['logoff','shutdown','critical','secret']});
 const rows=records(root);
 assert.equal(rows[1].event,'system-session-query');assert.deepEqual(rows[1].sessionReasons,['close-app']);
 assert.equal(rows[2].event,'system-session-end');assert.deepEqual(rows[2].sessionReasons,['shutdown','critical','logoff']);
 assert.ok(!fs.readFileSync(path.join(root,'lifecycle.jsonl'),'utf8').includes('secret'));
 log.finish(9,true);assert.equal(records(root).at(-1).clean,false);
 assert.equal(JSON.parse(fs.readFileSync(path.join(root,'session.json'),'utf8')).clean,false);
});
