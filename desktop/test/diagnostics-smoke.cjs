// Only an isolated Electron profile and fake Agent; no production process exits.
process.env.CXPALM_DESKTOP_FIXTURE='1';
const {app}=require('electron'),path=require('node:path'),fs=require('node:fs'),assert=require('node:assert/strict');
const mode=process.argv[2],profile=process.argv[3];
assert.ok(['clean','abrupt'].includes(mode));assert.ok(path.isAbsolute(profile));
app.setPath('userData',profile);app.disableHardwareAcceleration();
const desktop=require('../main.cjs');
(async()=>{
 await desktop.ready;const win=desktop.getWindow();
 win.close();assert.equal(win.isDestroyed(),false);assert.equal(win.isVisible(),false);
 const rows=fs.readFileSync(path.join(profile,'logs/lifecycle.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
 assert.ok(rows.some(row=>row.event==='startup-ready'));
 assert.ok(rows.some(row=>row.event==='window-hidden'));
 if(mode==='abrupt')app.exit(9);else app.quit();
})().catch(()=>app.exit(1));
