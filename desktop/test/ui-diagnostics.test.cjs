const test=require('node:test'),assert=require('node:assert/strict');
const {view}=require('../ui/diagnostics.js');
const now=1900000000000;
test('configured SSH and HTTPS never imply measured connectivity',()=>{
 const model=view({origin:'https://fixture.test',state:'stopped',owned:false,sshConfigured:true,desktopConnected:true},now);
 assert.equal(model.checkedAt,null);
 assert.deepEqual(model.rows.map(row=>row.id),['local','desktop','tunnel','public']);
 assert.equal(model.rows.some(row=>row.tone==='good'),false);
 assert.match(model.rows[2].value,/未验证/);assert.match(model.rows[3].value,/未探测/);
 assert.equal(model.rows[3].action,'open-web');assert.match(model.guidance,/官方任务不受影响/);
 const missing=view({},now);assert.equal(missing.rows[0].action,'settings');assert.equal(missing.rows[3].action,'settings');
});
test('only owned local runtime and a recent timed IPC check can be green',()=>{
 const state={state:'running',owned:true,origin:'https://fixture.test',desktopConnected:true,diagnosedAt:now-1000};
 const recent=view(state,now);
 assert.equal(recent.rows[0].tone,'good');assert.equal(recent.rows[1].tone,'good');assert.equal(recent.stale,false);
 assert.match(recent.guidance,/只读检查/);
 const stale=view({...state,diagnosedAt:now-300000},now);
 assert.equal(stale.rows[1].tone,'warn');assert.equal(stale.stale,true);assert.match(stale.guidance,/只读查看/);
 const unavailable=view({...state,desktopConnected:false},now);assert.equal(unavailable.rows[1].tone,'warn');assert.match(unavailable.rows[1].value,/不可连接/);
 const external=view({...state,state:'occupied',owned:false},now);assert.notEqual(external.rows[0].tone,'good');assert.match(external.rows[0].value,/其他进程/);
 for(const diagnosedAt of [undefined,null,0,-1,'1900000000000',NaN,Infinity,now+1]){
  const model=view({...state,diagnosedAt},now);assert.equal(model.checkedAt,null);assert.notEqual(model.rows[1].tone,'good');
 }
});
