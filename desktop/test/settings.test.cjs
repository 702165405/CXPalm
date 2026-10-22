const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),os=require('node:os');
const {connection,editConnection,atomicJSON,readJSON,loginOptions,saveStartup,shouldAutoStart,shouldShowOnStartup,validateConfig}=require('../settings.cjs');
function temporary(t){const root=fs.mkdtempSync(path.join(os.tmpdir(),'cxpalm-settings-'));t.after(()=>{assert.ok(path.resolve(root).startsWith(path.join(os.tmpdir(),'cxpalm-settings-')));fs.rmSync(root,{recursive:true,force:true});});return root;}
function mockApp(){let enabled=false;return {isPackaged:true,getPath:()=>'/test/CXPalm.exe',getLoginItemSettings:()=>({openAtLogin:enabled}),setLoginItemSettings:options=>{enabled=options.openAtLogin;}};}
test('first-run defaults, allowlist, stale-edit rejection and preserved private state',t=>{
 const root=temporary(t),file=path.join(root,'cxpalm.json');let current=connection(file,root);
 assert.equal(current.value.origin,'');assert.equal(fs.existsSync(file),false);
 assert.equal(current.value.codexHome,path.join(root,'.codex'));
 const input={revision:current.revision,origin:'https://example.test/',sshEnabled:true,sshHost:'host',sshUser:'deploy',sshPort:2222,sshRemotePort:18791};
 const edited=editConnection(current,input);assert.equal(edited.origin,'https://example.test');atomicJSON(file,{...edited,trustedProxyIP:true,custom:'preserved'});
 current=connection(file,root);assert.throws(()=>editConnection(current,input),/其他程序/);
 const updated=editConnection(current,{...input,revision:current.revision,sshEnabled:false});
 assert.equal(updated.sshHost,'');assert.equal(updated.sshPort,0);assert.equal(updated.sshUser,'');assert.equal(updated.sshRemotePort,0);
 assert.equal(updated.dataDir,current.value.dataDir);assert.equal(updated.trustedProxyIP,true);assert.equal(updated.custom,'preserved');
 assert.throws(()=>editConnection(current,{...input,revision:current.revision,dataDir:'/bad'}));
 assert.throws(()=>editConnection(current,{...input,revision:current.revision,sshRemotePort:80}));
 atomicJSON(file,updated);assert.equal(readJSON(file).value.custom,'preserved');assert.equal(fs.readdirSync(root).length,1);
});
test('login and app-open switches are independent; saving does not start a gateway',t=>{
 const root=temporary(t),file=path.join(root,'preferences.json'),app=mockApp();atomicJSON(file,{configPath:'/existing/config.json'});
 for(const login of [false,true])for(const open of [false,true]){
   saveStartup(app,file,{loginAutoStart:login,appAutoStart:open,startMinimized:false});
   assert.equal(app.getLoginItemSettings().openAtLogin,login);assert.equal(readJSON(file).value.appAutoStart,open);
   assert.equal(readJSON(file).value.configPath,'/existing/config.json');
   assert.equal(shouldAutoStart(true,login,open),login);assert.equal(shouldAutoStart(false,login,open),open);
 }
 assert.equal(shouldAutoStart(false,undefined,undefined),false);assert.equal(shouldAutoStart(true,undefined,true),false);
 assert.deepEqual(loginOptions(app,true),{openAtLogin:true,path:'/test/CXPalm.exe',args:['--login-start']});
 assert.throws(()=>saveStartup(app,file,{loginAutoStart:true,appAutoStart:false,startMinimized:false,command:'bad'}));
});
test('startup registration failure and preference failure never report success',t=>{
 const root=temporary(t),app=mockApp();app.setLoginItemSettings=()=>{};
 assert.throws(()=>saveStartup(app,path.join(root,'prefs.json'),{loginAutoStart:true,appAutoStart:false,startMinimized:false}),/系统未接受/);
 const rollback=mockApp(),bad=path.join(root,'bad.json');fs.writeFileSync(bad,'not JSON');
 assert.throws(()=>saveStartup(rollback,bad,{loginAutoStart:true,appAutoStart:true,startMinimized:false}));assert.equal(rollback.getLoginItemSettings().openAtLogin,false);
});
test('start-minimized preference persists independently and first-run errors stay visible',t=>{
 const root=temporary(t),file=path.join(root,'prefs.json'),app=mockApp();
 atomicJSON(file,{language:'en',configPath:'/private/config.json',unrelated:'keep'});
 for(const enabled of [true,false]){
  saveStartup(app,file,{loginAutoStart:false,appAutoStart:false,startMinimized:enabled});
  const saved=readJSON(file).value;
  assert.equal(saved.startMinimized,enabled);assert.equal(saved.language,'en');assert.equal(saved.unrelated,'keep');
  assert.equal(app.getLoginItemSettings().openAtLogin,false);assert.equal(saved.appAutoStart,false);
 }
 for(const loginLaunch of [false,true])for(const startMinimized of [false,true]){
  assert.equal(shouldShowOnStartup({loginLaunch,startMinimized,configured:true,failed:false}),!loginLaunch&&!startMinimized);
  assert.equal(shouldShowOnStartup({loginLaunch,startMinimized,configured:false,failed:false}),true);
  assert.equal(shouldShowOnStartup({loginLaunch,startMinimized,configured:true,failed:true}),true);
 }
 assert.equal(shouldShowOnStartup({configured:true}),true);
 for(const input of [{loginAutoStart:false,appAutoStart:false},{loginAutoStart:false,appAutoStart:false,startMinimized:'yes'}])assert.throws(()=>saveStartup(app,file,input));
});
test('real Go validates a connection without creating runtime state',async t=>{
 const root=temporary(t),binary=process.env.CXPALM_AGENT_TEST_BIN||path.resolve(__dirname,'../../.local/desktop-build/agent/cxpalm.exe');
 if(!fs.existsSync(binary)){t.skip('Build Go Agent first');return;}
 const value={listen:'127.0.0.1:18789',origin:'https://example.test',dataDir:path.join(root,'data'),codexHome:root,sshHost:'remote.test',sshUser:'deploy',sshPort:2222,sshRemotePort:18791};
 await validateConfig(binary,value);assert.equal(fs.existsSync(value.dataDir),false);
 for(const change of [{origin:'http://example.test'},{sshHost:'-oProxyCommand=bad'},{sshUser:'bad user'},{sshPort:70000},{listen:'0.0.0.0:18789'}])await assert.rejects(validateConfig(binary,{...value,...change}));
 assert.equal(fs.existsSync(value.dataDir),false);
});
