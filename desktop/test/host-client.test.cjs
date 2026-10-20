const test=require('node:test');
const assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {PassThrough}=require('node:stream');
const {HostClient,validate}=require('../host-client.cjs');
function fixture(){
 const child=new EventEmitter();child.stdin=new PassThrough();child.stdout=new PassThrough();child.stderr=new PassThrough();let call;
 const client=new HostClient('C:/app/agent.exe','C:/private/config.json',(...args)=>{call=args;return child;});
 return {client,child,call};
}
test('only fixed operations and device IDs can cross the desktop boundary',()=>{
 for(const method of ['status','start','stop','login.get','diagnose'])assert.doesNotThrow(()=>validate(method));
 assert.doesNotThrow(()=>validate('devices.revoke',{device:'a'.repeat(32)}));
 for(const args of [['pair.create',{}],['pair.status',{}],['pair.revoke',{}],['exec',{}],['status',{path:'secret'}],['start',{device:'a'.repeat(32)}],['devices.revoke',{device:'../../x'}],['status',[]]])assert.throws(()=>validate(...args));
});
test('host uses fixed executable arguments, no shell and correlated pipe replies',async()=>{
 const {client,child,call}=fixture();assert.equal(call[2].shell,false);assert.equal(call[2].windowsHide,true);assert.deepEqual(call[1],['desktop-host','--config','C:/private/config.json']);
 const waiting=client.request('status');const request=JSON.parse(child.stdin.read().toString());assert.equal(request.method,'status');
 const wire=JSON.stringify({id:request.id,data:{state:'stopped'}})+'\n';child.stdout.write(wire.slice(0,8));child.stdout.write(wire.slice(8));
 assert.deepEqual(await waiting,{state:'stopped'});client.fail('test complete');
});
test('malformed replies and child exit fail closed, writes are never replayed',async()=>{
 for(const wire of ['null\n','bad-json\n']){
  const {client,child}=fixture();const waiting=client.request('login.get');const rejected=assert.rejects(waiting);child.stdout.write(wire);await rejected;assert.equal(client.closed,true);assert.equal(client.pending.size,0);
 }
 const {client,child}=fixture();const waiting=client.request('start');const rejected=assert.rejects(waiting);child.emit('exit',1);await rejected;await assert.rejects(client.request('start'));
});
test('renderer stays sandboxed and cannot load remote scripts or connect directly',()=>{
 const fs=require('node:fs'),path=require('node:path');const main=fs.readFileSync(path.join(__dirname,'../main.cjs'),'utf8'),preload=fs.readFileSync(path.join(__dirname,'../preload.cjs'),'utf8'),html=fs.readFileSync(path.join(__dirname,'../ui/index.html'),'utf8');
 for(const flag of ['sandbox:true','contextIsolation:true','nodeIntegration:false','webSecurity:true'])assert.ok(main.includes(flag));
 assert.ok(main.includes("senderFrame !== win.webContents.mainFrame"));assert.ok(html.includes("connect-src 'none'"));assert.ok(html.includes("script-src 'self'"));assert.ok(!preload.includes('sendSync'));assert.ok(!preload.includes('require(\'node:'));
});

test('IP policy IPC accepts only bounded policy data, never arbitrary config',()=>{
 assert.doesNotThrow(()=>validate('ip-policy.set',{policy:{enabled:false,entries:[]}}));
 assert.doesNotThrow(()=>validate('ip-policy.set',{policy:{enabled:true,entries:['203.0.113.1']}}));
 for(const policy of [{enabled:true,entries:[]},{enabled:'yes',entries:[]},{enabled:true,entries:['']},{enabled:true,entries:['x'],command:'bad'}])assert.throws(()=>validate('ip-policy.set',{policy}));
 assert.throws(()=>validate('ip-policy.get',{policy:{enabled:false,entries:[]}}));
});

test('account settings IPC accepts only username/password/revision and does not return secrets',()=>{
 assert.doesNotThrow(()=>validate('login.get'));
 assert.doesNotThrow(()=>validate('login.set',{login:{username:'owner',password:'fixture-password-only',expected:''}}));
 for(const login of [{username:'owner',password:'short',expected:''},{username:'owner',password:'fixture-password-only',expected:'',command:'bad'},{username:'owner',password:'x'.repeat(257),expected:''}])assert.throws(()=>validate('login.set',{login}));
 assert.throws(()=>validate('status',{login:{username:'owner'}}));
});
