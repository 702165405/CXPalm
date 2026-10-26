// Real Electron window lifecycle with isolated preferences and a fake Agent.
process.env.CXPALM_DESKTOP_FIXTURE='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const mode=process.argv[2];
assert.ok(['visible','hidden','autostart-hidden','first-run','start-failed'].includes(mode));
const profile=path.resolve(__dirname,'../../.local/desktop-minimized-'+mode+'-'+process.pid);
app.setPath('userData',profile);app.disableHardwareAcceleration();
fs.mkdirSync(profile,{recursive:true});
fs.writeFileSync(path.join(profile,'desktop-preferences.json'),JSON.stringify({startMinimized:mode!=='visible',appAutoStart:['autostart-hidden','start-failed'].includes(mode)}));
if(mode==='first-run')process.env.CXPALM_DESKTOP_FIRST_RUN='1';
if(mode==='start-failed'){
 const fixture=require('./fixture.cjs'),create=fixture.create;
 fixture.create=()=>{const client=create(),request=client.request;client.request=async(method,args)=>{if(method==='start')throw Error('synthetic start failure');return request(method,args);};return client;};
}
const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow();
 assert.equal(win.isVisible(),['visible','first-run','start-failed'].includes(mode),'unexpected initial visibility');
 const state=await win.webContents.executeJavaScript("window.cxDesktop.invoke('status')",true);
 assert.equal(state.owned===true,mode==='autostart-hidden');
 if(mode==='start-failed')assert.match(state.startupError,/synthetic start failure/);
 if(mode==='hidden'){
  app.emit('second-instance',{},[],profile);
  assert.equal(win.isVisible(),true,'explicit reopening must restore hidden window');
 }
 console.log('PASS: isolated window startup '+mode+'; no real Agent or OS startup setting');
 await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
