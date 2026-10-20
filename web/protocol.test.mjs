import {test} from 'node:test';
import assert from 'node:assert/strict';
import {SnapshotStream,Bridge, conversation, approvalDecisions, contextUsage, StateSequence, NotificationInbox, sessionSettings, changedSettings, turnElapsed, activeQuestionIndex, computerAddress, extendHistory, mergeHistoryTurns, sessionIndicator, effortLabel, ReadingCache, offlineReading} from './protocol-v1.mjs';

const secret = 'a'.repeat(64);
test('conversation preserves text as text and approval choices fail closed', () => {
  const messages = conversation([{items:[{type:'userMessage',content:[{text:'<img onerror=alert(1)>'}]},
    {type:'agentMessage',text:'```js\ncode\n```',truncated:true},{type:'commandExecution',aggregatedOutput:'output'}]}]);
  assert.equal(messages[0].text, '<img onerror=alert(1)>');
  assert.equal(messages[1].truncated, true);
  assert.equal(messages[2].text, 'output');
  assert.deepEqual(approvalDecisions({method:'unknown/request'}), []);
  assert.deepEqual(approvalDecisions({method:'item/commandExecution/requestApproval',params:{availableDecisions:['decline']}}), ['decline']);
});

function fixture(existingStorage) {
  const data = existingStorage || new Map();
  const storage = {getItem:key=>data.get(key)??null,setItem:(key,value)=>data.set(key,value),removeItem:key=>data.delete(key)};
  const sockets=[];
  const bridge = new Bridge({storage,lock:async(_name,action)=>action(true),digest:async text=>`hash:${text}`,
    socket:()=>{const socket={readyState:1,sent:[],send(value){this.sent.push(JSON.parse(value));},close(){}};sockets.push(socket);return socket;}});
  bridge.connect({id:'device'});
  const hello=()=>sockets.at(-1).onmessage({data:JSON.stringify({type:'hello',data:{device:'device'}})});
  hello();
  const reply=(type,value)=>{const socket=sockets.at(-1),last=socket.sent.at(-1);socket.onmessage({data:JSON.stringify({id:last.id,type,[type==='error'?'error':'data']:value})});};
  return {bridge,data,sockets,reply,hello};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('attachment-only messages clear only confirmed attachment IDs, never newer files', async () => {
  const {bridge,sockets,reply}=fixture();
  const key='cxpalm:attachments:device:thread';
  bridge.storage.setItem(key,JSON.stringify([{id:'original'},{id:'newer'}]));
  const operation=bridge.mutate('chat.send',{thread:'thread',text:'',attachments:['original']});
  await tick();
  assert.deepEqual(sockets[0].sent[0].attachments,['original']);
  reply('result',{id:bridge.pending.id,status:'confirmed'});await operation;
  assert.deepEqual(JSON.parse(bridge.storage.getItem(key)),[{id:'newer'}]);
  const uncertain=bridge.mutate('chat.steer',{thread:'thread',text:'',attachments:['newer']});await tick();
  reply('result',{id:bridge.pending.id,status:'unknown'});await uncertain;
  assert.deepEqual(JSON.parse(bridge.storage.getItem(key)),[{id:'newer'}]);
});
test('writes persist before send and confirmed receipt clears only matching draft', async () => {
  const {bridge,sockets,reply}=fixture();
  const key=bridge.draftKey('thread');bridge.storage.setItem(key,'hello');
  const operation=bridge.mutate('chat.send',{thread:'thread',text:'hello'},{key,text:'hello'});
  await tick();
  const command=sockets[0].sent[0];assert.equal(bridge.pending.id,command.id);
  assert.equal(command.method,'chat.send');
  reply('result',{id:command.id,status:'confirmed'});
  await operation;
  assert.equal(bridge.pending,null);assert.equal(bridge.storage.getItem(key),null);
});
test('uncertain disconnect survives reload, reconnect does not replay, unknown requires explicit acknowledgement', async () => {
  const {bridge,data,sockets}=fixture();
  const operation=bridge.mutate('chat.send',{thread:'thread',text:'hello'});
  await tick();
  const original=bridge.pending.id;
  sockets[0].onclose();await assert.rejects(operation,/中断/);
  const next=fixture(data);assert.equal(next.bridge.pending.id,original);assert.equal(next.sockets[0].sent.length,0);
  await assert.rejects(next.bridge.mutate('chat.send',{thread:'thread',text:'hello'}),/未确认/);
  const check=next.bridge.checkPending();await tick();
  assert.equal(next.sockets[0].sent[0].method,'operation.get');
  next.reply('result',{id:original,status:'unknown'});await check;
  assert.equal(next.bridge.pending.id,original);
  const ack=next.bridge.acknowledge();await tick();next.reply('result',{id:original,status:'unknown'});
  await tick();assert.equal(next.sockets[0].sent.at(-1).method,'operation.acknowledge');
  next.reply('result',{id:original,status:'acknowledged'});await ack;
  assert.equal(next.bridge.pending,null);
});
test('other tabs and storage errors cannot dispatch writes; removed methods remain absent', async () => {
  const {bridge,sockets}=fixture();bridge.lock=async(_key,action)=>action(false);
  await assert.rejects(bridge.mutate('chat.send',{thread:'t',text:'x'}),/未确认/);
  assert.equal(sockets[0].sent.length,0);
  bridge.lock=async(_key,action)=>action(true);bridge.storage.setItem=()=>{throw new Error('storage unavailable');};
  await assert.rejects(bridge.mutate('chat.send',{thread:'t',text:'x'}),/storage/);
  assert.equal(sockets[0].sent.length,0);
  await assert.rejects(bridge.mutate('terminal.open',{}),/不支持/);
  await assert.rejects(bridge.mutate('files.read',{}),/不支持/);
});
test('newer draft and late receipts cannot erase new work',async()=>{
  const {bridge,reply}=fixture();const key=bridge.draftKey('t');bridge.storage.setItem(key,'first');
  const operation=bridge.mutate('chat.send',{thread:'t',text:'first'},{key,text:'first'});
  await tick();const id=bridge.pending.id;bridge.storage.setItem(key,'newer draft');
  reply('result',{id,status:'confirmed'});await operation;
  assert.equal(bridge.storage.getItem(key),'newer draft');
  bridge.storage.setItem(bridge.pendingKey,JSON.stringify({id:'new-operation'}));
  await assert.rejects(bridge.applyReceipt({id,status:'confirmed'}),/不匹配/);
  assert.equal(bridge.pending.id,'new-operation');
});

test('context uses actual last-turn input',()=>{
 assert.equal(contextUsage({}),null);assert.deepEqual(contextUsage({latestTokenUsageInfo:{last:{inputTokens:2500},modelContextWindow:10000}}),{used:2500,limit:10000,percent:25});
});
test('all workspace writes are removed, while settings and compact use durable receipts',async()=>{
 const {bridge,sockets}=fixture();for(const method of ['git.stage','git.commit','side.create','side.send'])await assert.rejects(bridge.mutate(method,{}),/不支持/);assert.equal(sockets[0].sent.length,0);
});

test('out-of-order cached states cannot replace newer live states; reconnect resets Agent counters',()=>{
 const order=new StateSequence();
 assert.equal(order.accept({id:'a',sequence:12}),true);
 assert.equal(order.accept({id:'b',sequence:1}),true);
 assert.equal(order.accept({id:'a',sequence:11}),false);
 assert.equal(order.accept({id:'a',sequence:12}),true);
 order.reset();assert.equal(order.accept({id:'a',sequence:0}),true);
});

test('notification inbox receives unselected chats, deduplicates replay and persists separate computer inboxes',()=>{
 const data=new Map();const storage={getItem:k=>data.get(k)||null,setItem:(k,v)=>data.set(k,v)};
 const a=new NotificationInbox(storage,'computer-a'),b=new NotificationInbox(storage,'computer-b');
 const event={id:'a'.repeat(64),kind:'approval',thread:'unselected',title:'Other conversation'};
 assert.equal(a.accept(event),true);assert.equal(a.accept(event),false);assert.equal(b.accept(event),true);
 assert.equal(new NotificationInbox(storage,'computer-a').accept(event),false);
 assert.equal(a.cursor,event.id);assert.equal(a.accept({...event,id:'b'.repeat(64),kind:'arbitrary'}),false);
});

test('two browser tabs refresh shared notification deduplication storage',()=>{
 const data=new Map();const storage={getItem:k=>data.get(k)||null,setItem:(k,v)=>data.set(k,v)};
 const first=new NotificationInbox(storage,'same-computer'),second=new NotificationInbox(storage,'same-computer');
 const event={id:'c'.repeat(64),kind:'complete',thread:'a',title:'Done'};
 assert.equal(first.accept(event),true);assert.equal(second.accept(event),false);
});

test('notification storage failure does not drop the alert or repeat it within the page',()=>{
 const inbox=new NotificationInbox({getItem:()=>null,setItem:()=>{throw new Error('quota');}},'computer');
 const event={id:'d'.repeat(64),kind:'complete',thread:'a',title:'Done'};
 assert.equal(inbox.accept(event),true);assert.equal(inbox.accept(event),false);
});

test('settings preselect actual configuration and submit only edited fields in the open group',()=>{
 const base=sessionSettings({latestModel:'actual',latestReasoningEffort:'high',controlsRevision:'original',latestThreadSettings:{activePermissionProfile:{id:':danger-full-access'}},latestCollaborationMode:{mode:'plan'}});
 assert.deepEqual(base,{model:'actual',effort:'high',permission:'full',mode:'plan',revision:'original'});
 assert.deepEqual(changedSettings(base,{...base},'permission'),{});
 assert.deepEqual(changedSettings(base,{...base,model:'new',permission:'workspace',mode:'default'},'model'),{model:'new'});
 assert.deepEqual(changedSettings(base,{...base,permission:'read-only'},'permission'),{permission:'read-only'});
 assert.deepEqual(changedSettings(base,{...base,mode:'default'},'plan'),{mode:'default'});
 assert.deepEqual(changedSettings(base,{...base,effort:'default'},'model'),{effort:'default'});
 const custom=sessionSettings({latestThreadSettings:{activePermissionProfile:{id:'custom-profile'}}});
 assert.equal(custom.permission,'');assert.equal(custom.model,'');assert.equal(custom.effort,'default');
 assert.equal(sessionSettings({currentPermissions:{sandboxPolicy:{type:'workspace-write'}}}).permission,'workspace');
 assert.equal(sessionSettings({latestThreadSettings:{activePermissionProfile:{id:':workspace'},approvalsReviewer:'auto_review'}}).permission,'auto-review');
 assert.equal(sessionSettings({latestThreadSettings:{activePermissionProfile:{id:':workspace'},approvalsReviewer:'user'}}).permission,'workspace');
 assert.deepEqual(changedSettings({permission:'workspace'},{permission:'auto-review'},'permission'),{permission:'auto-review'});
});

test('official turn durations and live elapsed never invent missing timestamps', () => {
  assert.equal(turnElapsed({status:'completed',durationMs:764351}, 1), '用时 12 分 44 秒');
  assert.equal(turnElapsed({status:'interrupted',durationMs:0}, 1), '用时 0 秒');
  assert.equal(turnElapsed({status:'inProgress',turnStartedAtMs:1000}, 66000, true), '执行中 · 1:05');
  assert.equal(turnElapsed({status:'inProgress',turnStartedAtMs:1000}, 66000, false), '');
  assert.equal(turnElapsed({status:'completed'}, 66000, true), '');
  assert.equal(turnElapsed({status:'completed',durationMs:-2}, 66000), '');
  assert.equal(turnElapsed({turnStartedAtMs:1000}, 500, true), '');
  assert.equal(turnElapsed({}, 66000, true), '');
});

test('read disconnect is not an uncertain write; stale socket events cannot close a replacement',async()=>{
 const {bridge,sockets}=fixture();
 const read=bridge.command('chat.history',{thread:'t'});
 const rejected=assert.rejects(read,error=>error.message==='连接中断，正在恢复连接。');
 const old=sockets[0];old.onclose();await rejected;
 bridge.connect({id:'device'});
 sockets[1].onmessage({data:JSON.stringify({type:'hello',data:{device:'device'}})});
 old.onclose();old.onmessage({data:JSON.stringify({type:'hello',data:{device:'wrong'}})});
 assert.equal(bridge.connected,true);
 assert.equal(sockets[1].sent.length,0);
 bridge.disconnect();
});

test('hello timeout releases a stalled connection, but never a healthy one',t=>{
 t.mock.timers.enable({apis:['setTimeout']});
 const {bridge,sockets}=fixture();
 bridge.connect({id:'device'});
 const stalled=sockets.at(-1);let closed=0;
 stalled.close=()=>{closed++;stalled.onclose();};
 t.mock.timers.tick(12000);
 assert.equal(closed,1);assert.equal(bridge.socket,null);
 bridge.connect({id:'device'});
 const healthy=sockets.at(-1);healthy.close=()=>{closed++;};
 healthy.onmessage({data:JSON.stringify({type:'hello',data:{device:'device'}})});
 t.mock.timers.tick(12000);
 assert.equal(closed,1);assert.equal(bridge.connected,true);
 bridge.disconnect();
});

test('notification resume checkpoints survive reconnect without erasing deduplication',()=>{
 const data=new Map(),storage={getItem:k=>data.get(k)??null,setItem:(k,v)=>data.set(k,v)};
 const first=new NotificationInbox(storage,'account'),second=new NotificationInbox(storage,'account');
 const event={id:'a'.repeat(64),kind:'complete',thread:'chat',title:'Done'};
 first.accept(event);first.checkpoint('baseline:current');
 assert.equal(first.cursor,'baseline:current');assert.equal(second.cursor,'baseline:current');
 assert.equal(new NotificationInbox(storage,'account').cursor,'baseline:current');
 assert.equal(new NotificationInbox(storage,'account').accept(event),false);
 first.checkpoint('b'.repeat(64));assert.equal(second.cursor,'b'.repeat(64));
 first.checkpoint('x'.repeat(129));assert.equal(first.cursor,'b'.repeat(64));
 first.checkpoint('');assert.equal(new NotificationInbox(storage,'account').cursor,'');
});

test('process projection preserves visible summaries, command/output and turn-qualified identity', () => {
  const messages = conversation([{turnId:'one',items:[
    {id:'tool',type:'commandExecution',command:'git status',aggregatedOutput:'full output',status:'inProgress'},
    {type:'reasoning',summary:['Visible',{text:'**summary**'}],encrypted_content:'SECRET',rawContent:['SECRET']},
    {type:'reasoning',encrypted_content:'SECRET'},
    {type:'agentMessage',text:'```sh\ngit status\n```'},
  ]},{turnId:'two',items:[{id:'tool',type:'commandExecution',command:'pwd',aggregatedOutput:''}]}]);
  assert.equal(messages.length,4);
  assert.equal(messages[0].command,'git status'); assert.equal(messages[0].text,'full output');
  assert.equal(messages[0].collapsible,true); assert.equal(messages[0].status,'inProgress');
  assert.equal(messages[1].text,'Visible\n**summary**'); assert.equal(messages[1].collapsible,true);
  assert.equal(messages[2].collapsible,false); assert.equal(messages[3].text,'pwd');
  assert.notEqual(messages[0].turn,messages[3].turn);
  assert(!JSON.stringify(messages).includes('SECRET'));
});

test('question navigator follows long replies and respects the loaded historical page', () => {
  const questions=Array.from({length:70},(_,i)=>({turn:'t'+i}));
  const rows=[{turn:'t35',top:-2000},{turn:'t36',top:-1000},{turn:'t37',top:500}];
  assert.equal(activeQuestionIndex(questions,rows,35,false,false),36);
  assert.equal(activeQuestionIndex(questions,rows,35,true,false),37);
  assert.equal(activeQuestionIndex(questions,rows,35,true,true),69);
  assert.equal(activeQuestionIndex(questions,[{turn:'t35',top:100}],35,false,false),35);
  assert.equal(activeQuestionIndex([],[],35,true,true),-1);
});
test('computer shortcuts accept HTTPS sites only and never pairing credentials', () => {
  assert.equal(computerAddress('https://desktop.example/cxpalm/'),'https://desktop.example');
  assert.equal(computerAddress(' https://desktop.example:8443/ '),'https://desktop.example:8443');
  for(const address of ['http://desktop.example','javascript:alert(1)','https://user:pass@desktop.example','https://desktop.example/?token=secret','https://desktop.example/#pair=secret','https://desktop.example/other'])assert.throws(()=>computerAddress(address));
});

test('continuous history preserves order, replaces overlaps and respects both boundaries', () => {
  const turn=n=>({turnId:`t${n}`,items:[]});
  const latest={offset:40,total:60,hasMore:false,turns:Array.from({length:20},(_,i)=>turn(i+40))};
  const older={offset:20,total:60,hasMore:true,turns:Array.from({length:20},(_,i)=>turn(i+20))};
  const window=extendHistory(latest,older);
  assert.equal(window.offset,20);assert.equal(window.hasMore,false);assert.equal(window.turns.length,40);
  assert.equal(window.turns[0].turnId,'t20');assert.equal(window.turns.at(-1).turnId,'t59');
  const repeated=extendHistory(window,{...older,offset:0,turns:Array.from({length:20},(_,i)=>turn(i))});
  assert.equal(repeated.offset,0);assert.equal(repeated.turns.length,60);assert.equal(repeated.hasMore,false);
  const update=mergeHistoryTurns(repeated.turns,[{...turn(59),status:'completed'},turn(60)]);
  assert.equal(update.length,61);assert.equal(update[59].status,'completed');
  assert.equal(extendHistory(older,latest).hasMore,false);
});

test('read idle and unknown chats are quiet while official unread and active work keep indicators', () => {
  for(const status of ['idle','completed','failed','interrupted','unknown']) {
    assert.equal(sessionIndicator(status,false),null);assert.equal(sessionIndicator(status,null),null);
  }
  assert.equal(sessionIndicator('unknown',true),'completed');
  assert.equal(sessionIndicator('completed',true),'completed');
  assert.equal(sessionIndicator('failed',true),'failed');
  assert.equal(sessionIndicator('interrupted',true),'interrupted');
  assert.equal(sessionIndicator('running',false),'running');
  assert.equal(sessionIndicator('waiting',false),'waiting');
  assert.equal(sessionIndicator('running',true,true),null);
  assert.equal(effortLabel('high'),'高');assert.equal(effortLabel('custom-effort'),'custom-effort');
});

test('tab reading cache isolates authorizations and never restores approval/read authority',()=>{
  const data=new Map(),storage={getItem:k=>data.get(k)||null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k)};
  const cache=new ReadingCache(storage,'device:expiry');
  const snapshot={turns:[{turnId:'t',items:[{type:'agentMessage',text:'saved answer'}]}],requests:[{id:'approval'}],hasUnreadTurn:true,readToken:'secret',threadRuntimeStatus:{type:'active'},latestCollaborationMode:{mode:'plan'}};
  cache.remember('t',{snapshot,historyPage:{offset:0,turns:[]}});cache.save();
  const row=new ReadingCache(storage,'device:expiry').get('t');assert.equal(row.snapshot.turns[0].items[0].text,'saved answer');assert.equal(row.snapshot.latestCollaborationMode.mode,'plan');
  assert.equal(row.snapshot.requests,undefined);assert.equal(row.snapshot.hasUnreadTurn,undefined);assert.equal(row.snapshot.readToken,undefined);assert.equal(row.snapshot.threadRuntimeStatus.type,'unknown');
  assert.equal(snapshot.requests.length,1);assert.equal(new ReadingCache(storage,'other-device').get('t'),null);
  cache.clear();assert.equal(new ReadingCache(storage,'device:expiry').get('t'),null);
  for(let i=0;i<20;i++)cache.remember('t'+i,{snapshot});assert.equal(cache.rows.size,8);
});

test('lightweight history indexes reuse visible turns and preserve continuous reverse scrolling', async () => {
  const {seedHistoryIndex,extendHistory}=await import('./protocol-v1.mjs');
  const snapshot={turns:[{turnId:'t9'},{turnId:'t10'}]};
  const page={turns:[],questions:[],offset:9,total:11,hasMore:false};
  const index=seedHistoryIndex(page,snapshot,true);
  assert.equal(index.turns,snapshot.turns);
  assert.equal(page.turns.length,0);
  const earlier={turns:[{turnId:'t8'}],questions:[],offset:8,total:11,hasMore:true};
  assert.deepEqual(extendHistory(index,earlier).turns.map(turn=>turn.turnId),['t8','t9','t10']);
  assert.equal(seedHistoryIndex(earlier,snapshot,false),earlier);
});


test('Go stream contract reconstructs exact snapshots and shares untouched messages', async()=>{
  const {readFile}=await import('node:fs/promises');
  const frames=JSON.parse(await readFile(new URL('../internal/server/testdata/stream-contract.json',import.meta.url),'utf8'));
  const stream=new SnapshotStream();stream.reset('thread','fixture-stream');
  let first;
  for(const [index,frame] of frames.entries()){
    const event=stream.accept(frame.event);assert.deepEqual(event.data,frame.expected);
    if(index===0)first=event.data.snapshot.turns[0].items[0];
    if(index===1)assert.equal(first,event.data.snapshot.turns[0].items[0]);
  }
  assert.equal(stream.accept({type:'chat.delta',data:{id:'thread',streamID:'old-stream',base:5,streamRevision:6,ops:[]}}),null);
  assert.throws(()=>stream.accept({type:'chat.delta',data:{id:'thread',streamID:'fixture-stream',base:99,streamRevision:100,ops:[]}}),/gap/);
  assert.equal(stream.accept(frames[1].event),null,'no further deltas accepted after a gap');
  stream.reset('thread','fixture-stream');assert.deepEqual(stream.accept(frames[0].event).data,frames[0].expected);
});
test('stream gap recovers by read-only subscription without replaying a pending write',async()=>{
  const {bridge,sockets,reply}=fixture();
  const subscription=bridge.command('chat.subscribe',{thread:'thread',stream:true});reply('result',{subscribed:true});await subscription;
  const id=sockets[0].sent.at(-1).id;
  sockets[0].onmessage({data:JSON.stringify({type:'chat.state',data:{id:'thread',streamID:id,streamRevision:1,sequence:1,snapshot:{turns:[]}}})});
  const operation=bridge.mutate('chat.send',{thread:'thread',text:'hello'});await tick();
  const write=sockets[0].sent.at(-1);
  sockets[0].onmessage({data:JSON.stringify({type:'chat.delta',data:{id:'thread',streamID:id,base:7,streamRevision:8,ops:[]}})});
  const recovery=sockets[0].sent.at(-1);assert.equal(recovery.method,'chat.subscribe');assert.equal(recovery.thread,'thread');
  assert.equal(sockets[0].sent.filter(q=>q.method==='chat.send').length,1);
  sockets[0].onmessage({data:JSON.stringify({type:'result',id:write.id,data:{id:write.id,status:'confirmed'}})});await operation;
  reply('result',{subscribed:true});bridge.disconnect();
});
test('deferred process rows remain visible and immutable offline cache reuses body objects',()=>{
  const item={id:'tool',type:'commandExecution',text:'',command:'echo fixture',deferred:true,bodyRevision:'v1'};
  const turns=[{turnId:'t',items:[item]}];const messages=conversation(turns);assert.equal(messages.length,1);assert.equal(messages[0].deferred,true);assert.equal(conversation(turns)[0],messages[0]);
  const snapshot={turns,requests:[{id:'approval'}],hasUnreadTurn:true,threadRuntimeStatus:{type:'active'}};
  const offline=offlineReading(snapshot);assert.equal(offline.turns,turns);assert.equal(snapshot.hasUnreadTurn,true);assert.equal(offline.hasUnreadTurn,undefined);
});

test('native imageGeneration items are assistant image-only messages',()=>{const rows=conversation([{turnId:'turn',items:[{id:'generated',type:'imageGeneration',images:[{id:'f'.repeat(64),name:'generated.png'}]}]}]);assert.equal(rows.length,1);assert.equal(rows[0].role,'Codex');assert.equal(rows[0].collapsible,false);assert.equal(rows[0].images.length,1);});

test('silent open socket expires without replaying writes and old callbacks cannot revive it',async()=>{
 const {bridge,sockets}=fixture();const socket=sockets[0];let lost=0;bridge.event=e=>{if(e.type==='disconnected')lost++};
 const operation=bridge.mutate('chat.send',{thread:'thread',text:'once'});await tick();const pending=bridge.pending.id;
 socket.onmessage({data:JSON.stringify({type:'connection.heartbeat'})});const last=bridge.lastMessageAt;
 bridge.checkConnection(last+29999);assert.equal(bridge.connected,true);
 bridge.checkConnection(last+30000);await assert.rejects(operation,/中断/);
 assert.equal(bridge.connected,false);assert.equal(bridge.pending.id,pending);assert.equal(lost,1);
 socket.onclose();socket.onmessage({data:JSON.stringify({type:'hello',data:{device:'device'}})});
 assert.equal(lost,1);assert.equal(bridge.connected,false);assert.equal(socket.sent.filter(q=>q.method==='chat.send').length,1);
 bridge.disconnect();
});
