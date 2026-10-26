// Local language persistence and rendering only. No real config, network or OS login changes.
process.env.CXPALM_DESKTOP_FIXTURE='1';process.env.CXPALM_DESKTOP_SMOKE='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const profile=process.env.CXPALM_LANGUAGE_PROFILE||path.resolve(__dirname,'../../.local/desktop-language-'+process.pid),persist=process.argv.includes('persist');
app.setPath('userData',profile);app.disableHardwareAcceleration();const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow(),output=path.resolve(__dirname,'../../.local/desktop-ui-tests');
 const evaluate=script=>win.webContents.executeJavaScript(script,true),click=id=>evaluate(`document.getElementById(${JSON.stringify(id)}).click()`);
 const wait=async predicate=>{const end=Date.now()+10000;while(Date.now()<end){if(await evaluate(predicate))return;await new Promise(r=>setTimeout(r,80));}throw new Error('Timeout: '+predicate);};
 const capture=async name=>{await new Promise(r=>setTimeout(r,200));fs.writeFileSync(path.join(output,name+'.png'),(await win.webContents.capturePage()).toPNG());};
 const change=async value=>{await evaluate(`document.getElementById('desktop-language').value=${JSON.stringify(value)};document.getElementById('desktop-language').dispatchEvent(new Event('change'))`);await wait(`document.documentElement.lang===${JSON.stringify(value)}&&!document.getElementById('desktop-language').disabled`);};
 await wait("document.getElementById('active-count').textContent==='13'");
 if(persist){assert.equal(await evaluate('document.documentElement.lang'),'en');assert.equal(await evaluate("document.querySelector('[data-page=settings]').textContent.includes('Settings')"),true);console.log('PASS: saved English restored on a separate fresh Electron launch');await desktop.closeForTest();return;}
 await evaluate("document.querySelector('[data-page=settings]').click()");await wait("!document.getElementById('setting-origin').disabled");
 const file=JSON.parse(fs.readFileSync(path.join(profile,'cxpalm.json'),'utf8'));
 const prefs=path.join(profile,'desktop-preferences.json');fs.writeFileSync(prefs,JSON.stringify({configPath:path.join(profile,'cxpalm.json'),appAutoStart:false,unrelated:'preserved'}));
 await evaluate("document.getElementById('setting-origin').value='https://unsaved.example.test';document.getElementById('setting-origin').dispatchEvent(new Event('input'));document.getElementById('setting-app-start').checked=true;document.getElementById('setting-app-start').dispatchEvent(new Event('change'))");
 await change('en');await wait("document.querySelector('[data-page=settings]').textContent.includes('Settings')");
 assert.equal(await evaluate("document.getElementById('setting-origin').value"),'https://unsaved.example.test');assert.equal(await evaluate("document.getElementById('setting-app-start').checked"),true);
 assert.equal(await evaluate("document.getElementById('save-startup').textContent"),'Save startup settings');assert.equal(await evaluate("document.getElementById('setting-host').getAttribute('placeholder')"),'server.example.com or my-server');
 assert.equal(await evaluate("window.cxDesktop.setLanguage('de').then(()=>false,()=>true)"),true);
 assert.equal(JSON.parse(fs.readFileSync(prefs)).unrelated,'preserved');assert.equal(JSON.parse(fs.readFileSync(prefs)).appAutoStart,false);assert.deepEqual(JSON.parse(fs.readFileSync(path.join(profile,'cxpalm.json'))),file);
 await capture('language-settings-en');
 for(const page of ['overview','connect','devices','diagnostics','updates']){
   await evaluate(`document.querySelector('[data-page=${page}]').click()`);
   if(page==='devices')await wait("document.querySelectorAll('.device-row').length===2");
   if(page==='updates'){await click('check-update');await wait("document.getElementById('update-phase').textContent==='Update available'");assert.ok((await evaluate("document.getElementById('update-notes').textContent")).includes('隔离更新演示'));}
   if(page==='devices')assert.ok((await evaluate("document.getElementById('devices-list').textContent")).includes('我的手机'));
   await capture('language-'+page+'-en');
   const han=await evaluate(`(()=>{const view=document.querySelector('[data-view=${page}]');const nodes=[];const w=document.createTreeWalker(view,NodeFilter.SHOW_TEXT);for(let n;n=w.nextNode();)if(/[\u4e00-\u9fff]/u.test(n.nodeValue)&&n.parentElement.getClientRects().length&&!n.parentElement.closest('.device-info,.device-log,#update-notes'))nodes.push(n.nodeValue);return nodes;})()`);
   assert.deepEqual(han,[],page+' untranslated visible application text');
 }
 win.setSize(900,740);await capture('language-updates-compact-en');await evaluate("document.querySelector('[data-page=overview]').click()");await capture('language-overview-compact-en');assert.equal(await evaluate('document.documentElement.scrollWidth<=innerWidth'),true);
 await evaluate("document.querySelector('[data-page=settings]').click()");await change('zh-CN');assert.equal(await evaluate("document.getElementById('save-startup').textContent"),'保存启动设置');
 await change('en');assert.equal(await evaluate("document.getElementById('global-status').textContent.includes('Agent stopped')"),true);
 console.log('PASS: en/zh round-trip, native preference preservation, unknown-language refusal, unsaved inputs retained, all six pages English, user names/release notes unchanged, 900px layout');await desktop.closeForTest();
})().catch(error=>{console.error(error);app.exit(1);});
