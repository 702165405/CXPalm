const test=require('node:test');const assert=require('node:assert/strict');
const {HostClient}=require('../host-client.cjs');const fs=require('node:fs/promises');const path=require('node:path');const net=require('node:net');const http=require('node:http');const {once}=require('node:events');
test('real Go stdio host: lifecycle, isolated account login/revocation and pipe shutdown',async t=>{
 const binary=process.env.CXPALM_AGENT_TEST_BIN||path.resolve(__dirname,'../../.local/desktop-build/agent',process.platform==='win32'?'cxpalm.exe':'cxpalm');
 try{await fs.access(binary);}catch{t.skip('Build desktop Agent first');return;}
 const root=await fs.mkdtemp(path.resolve(__dirname,'../../.local/desktop-process-test-'));
 const probe=net.createServer();probe.listen(0,'127.0.0.1');await once(probe,'listening');const port=probe.address().port;await new Promise(resolve=>probe.close(resolve));
 const config=path.join(root,'cxpalm.json');await fs.writeFile(config,JSON.stringify({listen:`127.0.0.1:${port}`,origin:'https://isolated.cxpalm.test',dataDir:root,codexHome:root,ipcPath:process.platform==='win32'?String.raw`\\.\pipe\cxpalm-account-test-${port}`:path.join(root,'missing-ipc.sock')}));
 const client=new HostClient(binary,config);t.after(()=>client.close());
 assert.equal((await client.request('status')).state,'stopped');assert.equal((await client.request('start')).state,'running');
 const post=(route,body,headers={})=>new Promise((resolve,reject)=>{
  const request=http.request({hostname:'127.0.0.1',port,path:route,method:'POST',headers:{Host:'isolated.cxpalm.test',Origin:'https://isolated.cxpalm.test','Content-Type':'application/json',...headers}},response=>{let text='';response.on('data',chunk=>text+=chunk);response.on('end',()=>{let value;try{value=JSON.parse(text);}catch{}resolve({status:response.statusCode,body:value,cookies:response.headers['set-cookie']||[]});});});request.on('error',reject);request.end(JSON.stringify(body));
 });
 assert.equal((await client.request('login.get')).configured,false);
 const password='fixture-password-only-12345';const login=await client.request('login.set',{login:{username:'owner',password,expected:''}});
 for(const method of ['pair.create','pair.status','pair.revoke'])await assert.rejects(client.request(method));
 for(const route of ['/api/v1/pair','/api/v1/browser/pair'])assert.equal((await post(route,{secret:'a'.repeat(64)})).status,404);
 assert.equal(login.configured,true);assert.ok(!JSON.stringify(login).includes(password));await assert.rejects(client.request('pair.create'));
 const native=await post('/api/v1/login',{username:'owner',password,remember:true});assert.equal(native.status,200);
 assert.equal(native.body.device.expires-native.body.device.created,30*24*3600);
 assert.equal((await post('/api/v1/browser/session',{}, {Authorization:'Bearer '+native.body.token})).status,200);
 const browser=await post('/api/v1/browser/login',{username:'owner',password,remember:false});assert.equal(browser.status,200);assert.equal(browser.body.token,undefined);
 assert.ok(browser.cookies[0].includes('HttpOnly'));assert.ok(!browser.cookies[0].includes('Expires='));
 const cookie=browser.cookies[0].split(';')[0];assert.equal((await post('/api/v1/browser/session',{}, {Cookie:cookie})).status,200);
 assert.equal((await post('/api/v1/logout',{}, {Authorization:'Bearer '+native.body.token})).status,200);
 assert.equal((await post('/api/v1/browser/session',{}, {Authorization:'Bearer '+native.body.token})).status,401);
 await client.request('login.set',{login:{username:'owner',password:password+'-new',expected:login.revision}});
 assert.equal((await post('/api/v1/browser/session',{}, {Cookie:cookie})).status,401);
 assert.equal((await post('/api/v1/login',{username:'owner',password,remember:true})).status,401);
 assert.equal((await client.request('stop')).state,'stopped');
 const settings=require('../settings.cjs'),current=settings.connection(config,root);
 const edited=settings.editConnection(current,{revision:current.revision,origin:'https://updated.cxpalm.test',sshEnabled:false,sshHost:'',sshUser:'',sshPort:0,sshRemotePort:18791});
 await settings.validateConfig(binary,edited);settings.atomicJSON(config,edited);
 const exited=once(client.child,'exit');await client.close();await exited;
 const reopened=new HostClient(binary,config);t.after(()=>reopened.close());
 assert.equal((await reopened.request('status')).origin,'https://updated.cxpalm.test');
 const preserved=await reopened.request('devices');assert.equal(preserved.length,2);assert.ok(preserved.every(device=>device.revoked));assert.equal((await reopened.request('login.get')).configured,true);
 assert.ok((await reopened.request('device.logs')).some(event=>event.action==='login'));await reopened.close();
});
