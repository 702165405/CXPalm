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
 win.emit('query-session-end',{reasons:['close-app','fixture-secret']});
 let sessionRows=fs.readFileSync(path.join(profile,'logs/lifecycle.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
 assert.deepEqual(sessionRows.filter(row=>row.event==='system-session-query').map(row=>row.sessionReasons),[['close-app']]);
 assert.equal(sessionRows.some(row=>row.event==='system-session-end'),false);
 win.emit('session-end',{reasons:['logoff']});
 sessionRows=fs.readFileSync(path.join(profile,'logs/lifecycle.jsonl'),'utf8').trim().split('\n').map(JSON.parse);
 assert.deepEqual(sessionRows.filter(row=>row.event==='system-session-end').map(row=>row.sessionReasons),[['logoff']]);
 assert.ok(!JSON.stringify(sessionRows).includes('fixture-secret'));
 if(mode==='abrupt')app.exit(9);else app.quit();
})().catch(()=>app.exit(1));
