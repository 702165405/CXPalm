// Fresh desktop profile: no real Agent, login registration or remote server.
process.env.CXPALM_DESKTOP_FIXTURE='1';process.env.CXPALM_DESKTOP_SMOKE='1';process.env.CXPALM_DESKTOP_FIRST_RUN='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const profile=path.resolve(__dirname,'../../.local/desktop-first-run-'+process.pid);app.setPath('userData',profile);app.disableHardwareAcceleration();
const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow();
 const evaluate=script=>win.webContents.executeJavaScript(script,true);
 const wait=async predicate=>{const end=Date.now()+10000;while(Date.now()<end){if(await evaluate(predicate))return;await new Promise(r=>setTimeout(r,80));}throw new Error(predicate);};
 await wait("document.getElementById('global-status').textContent.includes('尚未配置')");
 await evaluate("document.getElementById('hero-config').click()");
 await wait("!document.getElementById('setting-origin').disabled");
 assert.equal(await evaluate("document.getElementById('setting-login-start').checked||document.getElementById('setting-app-start').checked"),false);
 assert.equal(fs.existsSync(path.join(profile,'cxpalm.json')),false);
 assert.equal(await evaluate("window.cxDesktop.saveStartup({loginAutoStart:false,appAutoStart:true}).then(()=>false,()=>true)"),true);
 await evaluate("document.getElementById('setting-origin').value='https://fresh.example.test';document.getElementById('setting-origin').dispatchEvent(new Event('input'));document.getElementById('save-connection').click()");
 await wait("document.getElementById('global-status').textContent.includes('已停止')");
 const config=JSON.parse(fs.readFileSync(path.join(profile,'cxpalm.json'),'utf8'));
 assert.equal(config.origin,'https://fresh.example.test');assert.equal(config.dataDir,path.join(profile,'data'));assert.equal(config.sshHost,'');
 assert.equal(fs.existsSync(path.join(profile,'data')),false);
 console.log('PASS: fresh profile creates local config from UI, both startup defaults OFF, no OS registration, no Agent launch');await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
