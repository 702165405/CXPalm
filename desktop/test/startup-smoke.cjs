// Lifecycle branching test with mocked OS login items and a fake gateway.
process.env.CXPALM_DESKTOP_FIXTURE='1';process.env.CXPALM_DESKTOP_SMOKE='1';
const mode=process.argv[2],login=mode.startsWith('login-'),expected=mode.endsWith('-on');
if(login)process.argv.push('--login-start');
process.env.CXPALM_DESKTOP_FIXTURE_LOGIN=mode==='login-on'?'1':'0';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const profile=path.resolve(__dirname,'../../.local/desktop-startup-'+mode+'-'+process.pid);app.setPath('userData',profile);app.disableHardwareAcceleration();
fs.mkdirSync(profile,{recursive:true});fs.writeFileSync(path.join(profile,'desktop-preferences.json'),JSON.stringify({appAutoStart:mode==='app-on'||mode==='login-off'}));
const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow();const result=await win.webContents.executeJavaScript("window.cxDesktop.invoke('status')",true);
 assert.equal(result.owned,expected);assert.equal(result.state,expected?'running':'stopped');
 console.log('PASS: '+mode+' follows only its own startup switch; no actual OS registration or Agent launch');await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
