const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const {Readable,PassThrough}=require('node:stream'),{createHash}=require('node:crypto'),{EventEmitter}=require('node:events');
const {DesktopUpdater,FEED,MAX_SIZE,newer,validateManifest,request}=require('../updater.cjs');
const bytes=Buffer.from('MZ isolated test bytes, not an executable');
const manifest=()=>({schema:1,appId:'dev.cxpalm.desktop',platform:'win32',arch:'x64',version:'0.0.10',url:'https://win.lqilt.top/cxpalm/CXPalm-Setup-0.0.10-x64.exe',size:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),notes:'Isolated release notes <script>text only</script>'});
function setup(t,{raw=manifest(),body=bytes,headers,fetch}={}){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'cxpalm-updates-'));t.after(()=>{assert.ok(path.resolve(root).startsWith(path.join(os.tmpdir(),'cxpalm-updates-')));fs.rmSync(root,{recursive:true,force:true});});
 const calls=[];const updater=new DesktopUpdater({version:'0.0.9',cacheDir:root,platform:'win32',arch:'x64',fetch:fetch|| (async(url,signal)=>{calls.push(url);const stream=Readable.from([url===FEED?Buffer.from(JSON.stringify(raw)):body]);stream.headers=url===FEED?{}:headers||{};return stream;})});t.after(()=>updater.dispose());return {root,updater,calls};
}
test('stable numeric versions and platform/URL/hash/size trust boundaries',()=>{
 assert.equal(newer('0.1.0','0.0.10'),true);assert.equal(newer('0.0.10','0.0.9'),true);assert.equal(newer('1.10.0','1.9.9'),true);assert.equal(newer('0.0.9','0.0.10'),false);assert.equal(newer('0.0.9','0.0.9'),false);
 for(const version of ['v1.0.0','1.0.0-beta','01.0.0','1.0',null])assert.throws(()=>newer(version,'0.0.9'));
 assert.equal(validateManifest(manifest()).version,'0.0.10');
 for(const change of [{schema:2},{appId:'other'},{arch:'arm64'},{platform:'darwin'},{size:0},{size:MAX_SIZE+1},{size:1.2},{sha256:'bad'},{notes:'x'.repeat(8001)},{url:'http://win.lqilt.top/cxpalm/CXPalm-Setup-0.0.10-x64.exe'},{url:'https://evil.test/CXPalm.exe'},{url:manifest().url+'?'},{url:manifest().url+'#'},{url:manifest().url.replace('0.0.10','0.0.9')}])assert.throws(()=>validateManifest({...manifest(),...change}));
});
test('check does not download; automatic throttling, manual refresh and no downgrades',async t=>{
 const {updater,calls}=setup(t);assert.equal((await updater.check()).phase,'available');assert.deepEqual(calls,[FEED]);
 await updater.check(true);assert.equal(calls.length,1);await updater.check();assert.equal(calls.length,2);
 const restarted=new DesktopUpdater({version:'0.0.9',platform:'win32',arch:'x64',cacheDir:updater.cacheDir,fetch:()=>{throw new Error('throttled restart must not fetch');}});assert.equal((await restarted.check(true)).phase,'idle');
 const old=setup(t,{raw:{...manifest(),version:'0.0.8',url:manifest().url.replace('0.0.10','0.0.8')}}).updater;
 assert.equal((await old.check()).phase,'current');await assert.rejects(old.download());
 const unsupported=new DesktopUpdater({version:'0.0.9',cacheDir:updater.cacheDir,platform:'darwin',fetch:()=>{throw new Error('must not access network');}});
 assert.equal((await unsupported.check(true)).phase,'unsupported');await assert.rejects(unsupported.check());
});
test('download is size/SHA-256 checked, private path never returned; install rechecks disk',async t=>{
 const {updater,root}=setup(t);await updater.check();assert.equal((await updater.download()).phase,'ready');
 const file=await updater.installer();assert.equal(path.dirname(file),root);assert.deepEqual(fs.readFileSync(file),bytes);
 assert.equal(JSON.stringify(updater.status()).includes(root),false);assert.equal(JSON.stringify(updater.status()).includes('https:'),false);
 fs.writeFileSync(file,Buffer.alloc(bytes.length,97));await assert.rejects(updater.installer(),/重新下载/);assert.equal(updater.status().phase,'error');
 await updater.download();assert.equal(fs.readdirSync(root).filter(name=>name.endsWith('.exe')).length,1);assert.equal(updater.status().phase,'ready');
});
test('truncation, excess bytes, incorrect digest and declared length reject and remove partials',async t=>{
 for(const options of [{body:bytes.subarray(0,4)},{body:Buffer.concat([bytes,bytes])},{body:Buffer.alloc(bytes.length)},{headers:{'content-length':'123456'}}]){
   const {updater,root}=setup(t,options);await updater.check();assert.equal((await updater.download()).phase,'error');assert.equal(fs.readdirSync(root).filter(name=>name.endsWith('.part')||name.endsWith('.exe')).length,0);await assert.rejects(updater.installer());
 }
});
test('oversized/malformed manifests and connection errors are visible and fail closed',async t=>{
 for(const fetch of [async()=>Readable.from([Buffer.alloc(65537,32)]),async()=>Readable.from([Buffer.from('not JSON')]),async()=>{throw new Error('offline');}]){
   const {updater}=setup(t,{fetch});assert.equal((await updater.check()).phase,'error');assert.equal(updater.status().release,null);await assert.rejects(updater.download());
 }
});
test('download cancellation and overlap do not leave runnable files',async t=>{
 let stream,started;const ready=new Promise(resolve=>started=resolve);
 const {updater,root}=setup(t,{fetch:async url=>{
  if(url===FEED)return Readable.from([Buffer.from(JSON.stringify(manifest()))]);
  stream=new PassThrough();stream.headers={};stream.write(bytes.subarray(0,2));started();return stream;
 }});
 await updater.check();const pending=updater.download();await ready;
 await assert.rejects(updater.download());await assert.rejects(updater.check());assert.equal((await updater.check(true)).phase,'downloading');
 updater.cancel();await pending;assert.equal(updater.status().phase,'available');assert.equal(fs.readdirSync(root).filter(name=>name.endsWith('.part')||name.endsWith('.exe')).length,0);await assert.rejects(updater.installer());
});
test('native HTTPS rejects redirects, non-200 and compression; no cookies or tokens',async t=>{
 const https=require('node:https');let calls=0,options;
 t.mock.method(https,'get',(_url,input,callback)=>{calls++;options=input;const req=new EventEmitter();req.setTimeout=()=>req;queueMicrotask(()=>{const response=Readable.from([]);response.statusCode=calls===1?302:calls===2?404:200;response.headers=calls===3?{'content-encoding':'gzip'}:{};callback(response);});return req;});
 for(let i=0;i<3;i++)await assert.rejects(request(FEED,new AbortController().signal));
 assert.equal(calls,3);assert.equal(options.headers.Cookie,undefined);assert.equal(options.headers.Authorization,undefined);assert.equal(options.headers['Accept-Encoding'],'identity');
});

test('corrected 0.0.x release line rejects legacy feed without allowing a downgrade',async t=>{
 const {updater}=setup(t);updater.releaseLine='initial-001';
 assert.equal((await updater.check()).phase,'error');assert.equal(updater.status().release,null);await assert.rejects(updater.download());
 const current=setup(t,{raw:{...manifest(),releaseLine:'initial-001'}}).updater;current.releaseLine='initial-001';
 assert.equal((await current.check()).phase,'available');
 assert.throws(()=>validateManifest({...manifest(),releaseLine:'../legacy'}));
});

test('initial release build namespace is immutable and stays on the exact trusted HTTPS path',()=>{
 const release={...manifest(),releaseLine:'initial-001',build:36,url:'https://win.lqilt.top/cxpalm/CXPalm-Setup-0.0.9-build36-x64.exe'};
 release.version='0.0.9';
 assert.equal(validateManifest(release).build,36);
 for(const change of [{build:0},{build:'36'},{releaseLine:'preview-001'},{url:'https://evil.example/CXPalm-Setup-0.0.9-build36-x64.exe'},{url:'https://win.lqilt.top/cxpalm/../CXPalm-Setup-0.0.9-build36-x64.exe'}])assert.throws(()=>validateManifest({...release,...change}));
});
