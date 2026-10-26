// Native shell and renderer acceptance with fake update bytes; never executes an installer.
process.env.CXPALM_DESKTOP_FIXTURE='1';process.env.CXPALM_DESKTOP_SMOKE='1';process.env.CXPALM_DESKTOP_FIRST_RUN='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs/promises'),assert=require('node:assert/strict');
app.setPath('userData',path.resolve(__dirname,'../../.local/desktop-update-profile-'+process.pid));app.disableHardwareAcceleration();const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow(),output=path.resolve(__dirname,'../../.local/desktop-ui-tests');await fs.mkdir(output,{recursive:true});
 const evaluate=script=>win.webContents.executeJavaScript(script,true),click=id=>evaluate(`document.getElementById(${JSON.stringify(id)}).click()`);
 const wait=async predicate=>{const end=Date.now()+10000;while(Date.now()<end){if(await evaluate(predicate))return;await new Promise(r=>setTimeout(r,80));}throw new Error(predicate);};
 const capture=async name=>{await new Promise(r=>setTimeout(r,250));await fs.writeFile(path.join(output,name+'.png'),(await win.webContents.capturePage()).toPNG());};
 await wait("document.getElementById('global-status').textContent.includes('尚未配置')");
 await evaluate("document.querySelector('[data-page=updates]').click()");await click('check-update');await wait("document.getElementById('update-phase').textContent==='发现新版本'");
 assert.equal(await evaluate("document.getElementById('update-current').textContent"),'v'+require('../package.json').version);assert.equal(await evaluate("document.getElementById('update-latest').textContent"),'v0.1.1');assert.equal(await evaluate("document.querySelectorAll('#update-notes img,#update-notes script').length"),0);
 assert.ok((await evaluate("document.getElementById('update-notes').textContent")).includes('<img'));assert.equal(await evaluate("document.getElementById('nav-update').textContent"),'NEW');await capture('updates-available');
 await click('download-update');await wait("document.getElementById('update-phase').textContent==='等待安装'");assert.equal(await evaluate("document.getElementById('install-update').hidden"),false);
 assert.equal(await evaluate("window.cxDesktop.installUpdate().then(()=>false,()=>true)"),true);assert.equal(await evaluate("document.getElementById('global-status').textContent.includes('尚未配置')"),true);await capture('updates-ready');
 win.setSize(900,740);await capture('updates-compact');assert.equal(await evaluate('document.documentElement.scrollWidth<=window.innerWidth'),true);
 const ids=await evaluate("[...document.querySelectorAll('[id]')].map(n=>n.id)");assert.equal(ids.length,new Set(ids).size);
 console.log('PASS: update UI before configuration, check/download/verified-ready, safe release notes, native fixture install refusal, 900px layout; no live network or executable launch');await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
