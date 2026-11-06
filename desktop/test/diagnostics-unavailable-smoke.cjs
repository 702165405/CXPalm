process.env.CXPALM_DESKTOP_FIXTURE='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const profile=path.resolve(__dirname,'../../.local/desktop-diagnostics-unavailable-'+process.pid);
app.setPath('userData',profile);app.disableHardwareAcceleration();
fs.mkdirSync(profile,{recursive:true});
fs.writeFileSync(path.join(profile,'logs'),'fixture log path is not a directory');
fs.writeFileSync(path.join(profile,'desktop-preferences.json'),JSON.stringify({startMinimized:true}));
let showRequested=false;
app.on('browser-window-created',(_event,window)=>{const show=window.show.bind(window);window.show=()=>{showRequested=true;return show();};});
const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow();
 const state=await win.webContents.executeJavaScript("window.cxDesktop.invoke('status')",true);
 assert.equal(state.state,'stopped');assert.match(state.startupError,/诊断日志无法写入/);
 assert.equal(showRequested,true);
 assert.equal(fs.readFileSync(path.join(profile,'logs'),'utf8'),'fixture log path is not a directory');
 console.log('PASS: unavailable diagnostics stay visible without preventing isolated host connection or changing permissions');
 await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
