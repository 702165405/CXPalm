export class SnapshotStream {
  reset(thread=null,id=null){this.thread=thread;this.id=id;this.current=null;this.revision=0;this.broken=false;}
  constructor(){this.reset();}
  accept(event){
    if(!['chat.state','chat.delta'].includes(event.type))return event;
    const data=event.data;
    if(this.thread&&data.id!==this.thread || data.streamID&&this.id&&data.streamID!==this.id)return null;
    if(event.type==='chat.state'){
      this.current=data;this.revision=data.streamRevision||0;this.broken=false;return event;
    }
    if(this.broken)return null;
    if(!this.current||data.base!==this.revision||data.streamRevision!==this.revision+1){this.broken=true;throw new Error('snapshot stream gap');}
    try{
      let value=this.current;
      for(const op of data.ops||[])value=applyStreamOp(value,op);
      value={...value,streamRevision:data.streamRevision};this.current=value;this.revision=data.streamRevision;
      return {type:'chat.state',data:value};
    }catch(error){this.broken=true;throw error;}
  }
}
function applyStreamOp(root,op){
  if(!Array.isArray(op.path)||op.path.length>24||!op.path.every(key=>typeof key==='string'))throw new Error('invalid stream path');
  function apply(value,depth){
    if(depth===op.path.length){
      if(op.op==='set')return op.value??null;
      if(op.op==='append'&&typeof value==='string'&&typeof op.value==='string')return value+op.value;
      if(op.op==='resize'&&Array.isArray(value)&&Number.isInteger(op.value)&&op.value>=0&&op.value<=value.length)return value.slice(0,op.value);
      throw new Error('invalid stream operation');
    }
    if(!value||typeof value!=='object')throw new Error('missing stream parent');
    const key=op.path[depth],array=Array.isArray(value);
    if(array&&(!/^(0|[1-9][0-9]*)$/.test(key)||Number(key)>value.length))throw new Error('invalid array index');
    if(depth===op.path.length-1&&op.op==='remove'){
      if(array)throw new Error('array removal requires resize');
      const next={...value};delete next[key];return next;
    }
    const next=apply(Object.hasOwn(value,key)?value[key]:undefined,depth+1);
    if(array){const result=value.slice();result[Number(key)]=next;return result;}
    return {...value,[key]:next};
  }
  return apply(root,0);
}
const conversationCache=new WeakMap();
export const mutationMethods = new Set([
  'chat.send', 'chat.steer', 'chat.queue', 'chat.queue.cancel', 'chat.stop', 'chat.approve',
  'chat.settings', 'chat.goal', 'chat.compact', 'chat.notifications', 'sessions.create', 'sessions.rename', 'sessions.archive', 'sessions.unarchive',
]);

export function conversation(turns = []) {
  return (Array.isArray(turns) ? turns : []).flatMap((turn, turnIndex) => {
    const cached=conversationCache.get(turn);if(cached?.index===turnIndex)return cached.messages;
    const collection = turn.items;
    const items = Array.isArray(collection) ? collection :
      (collection?.islands || []).flatMap(island => (island.entries || [])
        .map(entry => collection.entitiesByKey?.[entry.value]).filter(Boolean));
    const messages=items.map((item, itemIndex) => {
      let text = item.text;
      if (!text && typeof item.content === 'string') text = item.content;
      if (!text && Array.isArray(item.content)) text = item.content.map(part => part.text || '').filter(Boolean).join('\n');
      if (item.type === 'reasoning') {
        const summary = typeof item.summary === 'string' ? item.summary :
          (Array.isArray(item.summary) ? item.summary.map(part => typeof part === 'string' ? part : part?.text || '').filter(Boolean).join('\n') : '');
        text = summary || text;
      }
      text ||= item.aggregatedOutput || item.command || '';
      return {id: item.id || `${item.type}:${itemIndex}`, turn: turn.turnId || turn.id || `turn-${turnIndex}`,
        command: typeof item.command === 'string' ? item.command : '', status: item.status,
        images:Array.isArray(item.images)?item.images:[],
        collapsible: !['userMessage', 'agentMessage','imageGenerationCall','imageGeneration'].includes(item.type), role: item.type === 'userMessage' ? '你' : ['agentMessage','imageGenerationCall','imageGeneration'].includes(item.type) ? 'Codex' : item.type,
        text: typeof text === 'string' ? text : '', truncated: item.truncated === true, deferred:item.deferred===true, bodyRevision:item.bodyRevision||''};
    }).filter(item => item.text||item.deferred||item.images.length);
    conversationCache.set(turn,{index:turnIndex,messages});return messages;
  });
}

export function approvalDecisions(request) {
  if(request.method==='mcpServer/elicitation/request') {
    const p=request.params,s=p?.requestedSchema;
    if(p?.mode!=='form'||p?._meta?.codex_approval_kind!=='mcp_tool_call'||s?.type!=='object'||!s.properties||typeof s.properties!=='object'||Array.isArray(s.properties)||Object.keys(s.properties).length)return [];
    if(Object.keys(s).some(key=>!['type','properties','required','title','description'].includes(key))||('required' in s&&(!Array.isArray(s.required)||s.required.length)))return [];
    const persist=p._meta.persist;
    const session=persist==='session'||Array.isArray(persist)&&persist.includes('session');
    return ['accept',...(session?['acceptForSession']:[]),'decline','cancel'];
  }
  if(request.method==='item/permissions/requestApproval') {
    const permissions=request.params?.permissions;
    if(!permissions||typeof permissions!=='object'||Array.isArray(permissions))return [];
    return Object.keys(permissions).every(key=>['network','fileSystem'].includes(key))?['accept','decline']:['decline'];
  }
  if (!['item/commandExecution/requestApproval', 'item/fileChange/requestApproval'].includes(request.method)) return [];
  const available = request.params?.availableDecisions;
  return ['accept', 'acceptForSession', 'decline', 'cancel'].filter(value => available == null ? ['accept','decline'].includes(value) : Array.isArray(available) && available.includes(value));
}

export function contextUsage(snapshot) {
  const usage = snapshot?.latestTokenUsageInfo;
  const used = usage?.last?.inputTokens;
  const limit = usage?.modelContextWindow;
  if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return null;
  return {used, limit, percent: Math.min(100, Math.max(0, Math.round(used / limit * 100)))};
}
export class Bridge {
  constructor({storage, lock, socket, changed = () => {}, event = () => {}, digest = async text => {
    const bytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
    return Array.from(new Uint8Array(bytes), byte => byte.toString(16).padStart(2, '0')).join('');
  }}) {
    Object.assign(this, {storage, lock, socketFactory: socket, changed, event, digest});
    this.calls = new Map();
    this.stream=new SnapshotStream();
    this.connected = false;
    this.device = null;
    this.socket = null;
    this.receipt = null;
    this.failures = new Map();
  }
  get pendingKey() { return `cxpalm:pending:${this.device.id}`; }
  get pendings() {
    const raw = this.device && this.storage.getItem(this.pendingKey);
    const saved = raw ? JSON.parse(raw) : [];
    return Array.isArray(saved) ? saved : [saved];
  }
  get pending() { return this.pendings[0] || null; }
  pendingFor(thread) { return this.pendings.find(op => (op.thread || null) === (thread || null)) || null; }
  noticeFor(thread) { return this.pendingFor(thread) || this.failures.get(thread || null) || null; }
  dismissFailure(id) {
    for(const [scope,operation] of this.failures)if(operation.id===id)this.failures.delete(scope);
    this.changed();
  }
  savePending(operation, remove = false) {
    const operations = this.pendings.filter(op => op.id !== operation.id);
    if (!remove) operations.push(operation);
    if (operations.length) this.storage.setItem(this.pendingKey, JSON.stringify(operations));
    else this.storage.removeItem(this.pendingKey);
  }
  receiptFor(pending) { return pending?.receipt || null; }
  draftKey(thread, side = false) { return `cxpalm:draft:${this.device.id}:${side ? 'side' : 'main'}:${thread}`; }
  connect(device) {
    this.disconnect();
    this.stream.reset();
    this.lastMessageAt=Date.now();
    if(this.device?.id!==device.id)this.failures.clear();
    this.device = device;
    this.receipt = null;
    const socket = this.socket = this.socketFactory();
    this.helloTimer = setTimeout(() => { if (this.socket === socket && !this.connected) socket.close(); }, 12000);
    this.helloTimer.unref?.();
    this.watchdog=setInterval(()=>this.checkConnection(),5000);this.watchdog.unref?.();
    socket.onmessage = message => {
      if (this.socket !== socket) return;
      let event;
      try { event = JSON.parse(message.data); } catch { socket.close(1003, 'Invalid JSON'); return; }
      this.lastMessageAt=Date.now();
      try{event=this.stream.accept(event);if(!event)return;}catch{const thread=this.stream.thread;if(thread)this.command('chat.subscribe',{thread,stream:true}).catch(()=>{});return;}
      if (event.type === 'hello') {
        if (event.data?.device !== this.device.id) { socket.close(1008, 'Device mismatch'); return; }
        clearTimeout(this.helloTimer);
        this.connected = true;
        this.changed();
      }
      const call = this.calls.get(event.id);
      if (call) {
        clearTimeout(call.timer);
        this.calls.delete(event.id);
        if (event.type === 'error') call.reject(new Error(event.error));
        else call.resolve(event.data);
      }
      this.event(event);
    };
    socket.onclose = () => this.lost(socket);
    socket.onerror = () => {};
  }
  checkConnection(now=Date.now()) {
    const socket=this.socket;
    if(!socket||!this.connected||now-this.lastMessageAt<30000)return;
    this.lost(socket);socket.close(4000,'Connection heartbeat expired');
  }
  lost(socket) {
    if(this.socket!==socket)return;
    clearTimeout(this.helloTimer);clearInterval(this.watchdog);
    this.socket=null;this.connected=false;this.rejectCalls();this.changed();this.event({type:'disconnected'});
  }
  rejectCalls() {
    for (const call of this.calls.values()) { clearTimeout(call.timer); call.reject(new Error(mutationMethods.has(call.method) ? '连接中断。写操作结果可能未知，不会自动重发。' : '连接中断，正在恢复连接。')); }
    this.calls.clear();
  }
  disconnect() {
    clearTimeout(this.helloTimer);clearInterval(this.watchdog);
    const old = this.socket;
    this.socket = null;
    this.connected = false;
    old?.close();
    this.rejectCalls();
  }
  command(method, fields = {}, id = crypto.randomUUID()) {
    if (!this.connected || this.socket?.readyState !== 1) return Promise.reject(new Error('请先连接电脑。'));
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.calls.delete(id); reject(new Error(mutationMethods.has(method) ? '等待超时。写操作请查询原结果，不要重复发送。' : '读取超时，请重试。')); }, 60000);
      this.calls.set(id, {resolve, reject, timer, method});
      if(method==='chat.subscribe')this.stream.reset(fields.thread,id);
      try { this.socket.send(JSON.stringify({...fields, id, method})); }
      catch (error) { clearTimeout(timer); this.calls.delete(id); reject(error); }
    });
  }
  async applyReceipt(receipt) {
    return this.lock(`cxpalm:write:${this.device.id}`, async acquired => {
    if (!acquired) throw new Error('另一个标签页正在核对回执，请重试。');
    const pending = this.pendings.find(op => op.id === receipt.id);
    if (!pending) throw new Error('操作回执不匹配。');
    this.receipt = receipt;
    if (['confirmed', 'acknowledged', 'not_sent', 'rejected'].includes(receipt.status)) {
      if(receipt.status==='confirmed'&&pending.attachmentKey&&pending.attachmentIDs){
        const files=JSON.parse(this.storage.getItem(pending.attachmentKey)||'[]');
        this.storage.setItem(pending.attachmentKey,JSON.stringify(files.filter(file=>!pending.attachmentIDs.includes(file.id))));
      }
      if (receipt.status === 'confirmed' && pending.draftKey && pending.draftDigest) {
        const text = this.storage.getItem(pending.draftKey);
        if (text !== null && await this.digest(text) === pending.draftDigest &&
            this.storage.getItem(pending.draftKey) === text && this.pendings.some(op => op.id === pending.id)) this.storage.removeItem(pending.draftKey);
      }
      // A late response must not clear another tab's newer pending operation.
      if (this.pendings.some(op => op.id === pending.id)) this.savePending(pending, true);
      if(['rejected','not_sent'].includes(receipt.status))this.failures.set(pending.thread||null,{...pending,receipt});
    } else this.savePending({...pending, receipt});
    this.changed();
    return receipt;
    }, true);
  }
  async mutate(method, fields, draft) {
    if (!mutationMethods.has(method)) throw new Error('不支持此写操作。');
    if (['chat.send','chat.steer'].includes(method) && (typeof fields.text!=='string' || (!fields.text.trim()&&!fields.attachments?.length) || new TextEncoder().encode(fields.text).length>100000)) throw new Error('消息不能为空且 UTF-8 长度不能超过 100,000 字节。');
    const draftDigest = draft ? await this.digest(draft.text) : null;
    // Hold the shared storage lock only while reserving an operation, not across network I/O.
    const pending = await this.lock(`cxpalm:write:${this.device.id}`, async acquired => {
      if (!acquired || this.pendingFor(fields.thread)) throw new Error('此会话还有未确认的操作，请先查看原操作结果。');
      if (!this.connected) throw new Error('请先连接电脑。');
      const pending = {id: crypto.randomUUID(), method, thread: fields.thread, created: Date.now()};
      if(fields.attachments?.length){pending.attachmentKey=`cxpalm:attachments:${this.device.id}:${fields.thread}`;pending.attachmentIDs=fields.attachments;}
      if (draft) { pending.draftKey = draft.key; pending.draftDigest = draftDigest; }
      this.savePending(pending);
      this.failures.delete(fields.thread||null);
      this.receipt = null;
      this.changed();
      return pending;
    });
    try { return await this.applyReceipt(await this.command(method, fields, pending.id)); }
    catch (error) {
      if (!this.pendings.some(op => op.id === pending.id)) throw error;
      await this.applyReceipt({id:pending.id,status:'unknown',result:{error:error.message}});
      throw Object.assign(error,{operation:pending.id});
    } finally { this.changed(); }
  }
  async checkPending(id = this.pending?.id) {
    const pending = this.pendings.find(op => op.id === id);
    if (!pending) return null;
    try { return await this.applyReceipt(await this.command('operation.get', {requestId: pending.id})); }
    catch(error) { throw Object.assign(error,{operation:pending.id}); }
  }
  async acknowledge(id = this.pending?.id) {
    try {
      const receipt = await this.checkPending(id);
      if (receipt?.status !== 'unknown') throw new Error('只能在人工核对后解除未知结果；正在执行的操作不能解除。');
      return await this.applyReceipt(await this.command('operation.acknowledge', {requestId: receipt.id, text: 'inspected'}));
    } catch(error) { throw Object.assign(error,{operation:id}); }
  }
}

// Sequence counters belong to one Agent connection, not to wall-clock time.
export class StateSequence {
  constructor(){this.values=new Map();}
  reset(){this.values.clear();}
  accept(state){
    if(!Number.isSafeInteger(state.sequence))return true;
    const previous=this.values.get(state.id);
    if(previous!==undefined && state.sequence<previous)return false;
    this.values.set(state.id,state.sequence);return true;
  }
}

export class NotificationInbox {
  constructor(storage,key){this.storage=storage;this.key=key;this.volatile=false;this.seen=[];try{const ids=JSON.parse(storage.getItem(key)||'[]');if(Array.isArray(ids))this.seen=ids.filter(id=>typeof id==='string').slice(-256);}catch{}}
  get cursor(){try{return this.storage.getItem(this.key+':cursor')??this.position??this.seen.at(-1)??'';}catch{return this.position??this.seen.at(-1)??'';}}
  checkpoint(cursor){if(typeof cursor!=='string'||cursor.length>128)return;this.position=cursor;try{this.storage.setItem(this.key+':cursor',cursor);}catch{}}
  accept(event){
    // Refresh shared storage inside the caller's cross-tab notification lock.
    try{const saved=this.volatile?null:JSON.parse(this.storage.getItem(this.key)||'[]');if(Array.isArray(saved))this.seen=saved.filter(id=>typeof id==='string').slice(-256);}catch{}
    if(!event||!/^[a-f0-9]{64}$/.test(event.id)||!['complete','approval'].includes(event.kind)||typeof event.thread!=='string'||!event.thread||event.thread.length>128||typeof event.title!=='string'||event.title.length>640||this.seen.includes(event.id))return false;
    this.checkpoint(event.id);this.seen.push(event.id);this.seen=this.seen.slice(-256);try{this.storage.setItem(this.key,JSON.stringify(this.seen));}catch{this.volatile=true;}return true;
  }
}

export function sessionSettings(snapshot = {}) {
  const settings=snapshot.latestThreadSettings||snapshot.currentPermissions||{};
  const id=settings.activePermissionProfile?.id||settings.permissions||settings.sandboxPolicy?.type;
  let permission=({':read-only':'read-only','read-only':'read-only',':workspace':'workspace','workspace':'workspace','workspace-write':'workspace',':danger-full-access':'full','danger-full-access':'full','full':'full'})[id]||'';
  if(permission==='workspace' && settings.approvalsReviewer==='auto_review')permission='auto-review';
  return {model:snapshot.latestModel||snapshot.latestThreadSettings?.model||'',effort:snapshot.latestReasoningEffort||snapshot.latestThreadSettings?.effort||'default',permission,mode:snapshot.latestCollaborationMode?.mode||'default',revision:snapshot.controlsRevision||''};
}
export function changedSettings(base,draft,group) {
  const fields=({model:['model','effort'],permission:['permission'],plan:['mode']})[group]||[];
  return Object.fromEntries(fields.filter(key=>draft[key]&&draft[key]!==base[key]).map(key=>[key,draft[key]]));
}

export function newChatDefaults(snapshot) {
  const {model,effort,permission}=sessionSettings(snapshot);
  if(typeof model!=='string'||!model||model.length>200||typeof effort!=='string'||effort.length>32||!['read-only','workspace','auto-review','full'].includes(permission))return null;
  const preference=snapshot?.notificationPreference;
  return {options:{model,effort,permission},...(preference?.available!==false&&typeof preference?.enabled==='boolean'?{enabled:preference.enabled}:{})};
}

// Official turn duration includes tool execution and approval waits, not token speed.
export function turnElapsed(turn, now, live = false) {
  const duration = turn?.durationMs;
  const ended = ['completed', 'failed', 'interrupted'].includes(turn?.status);
  let ms;
  if (ended && Number.isFinite(duration) && duration >= 0) ms = duration;
  else if (turn?.status === 'inProgress' && live && Number.isFinite(turn?.turnStartedAtMs) && turn.turnStartedAtMs > 0 && Number.isFinite(now) && now >= turn.turnStartedAtMs) ms = now - turn.turnStartedAtMs;
  else return '';
  const seconds = Math.floor(ms / 1000), minutes = Math.floor(seconds / 60), hours = Math.floor(minutes / 60);
  const value = hours ? `${hours} 小时 ${minutes % 60} 分 ${seconds % 60} 秒` : minutes ? `${minutes} 分 ${seconds % 60} 秒` : `${seconds} 秒`;
  if(ended)return `用时 ${value}`;
  const clock=hours?`${hours}:${String(minutes%60).padStart(2,'0')}:${String(seconds%60).padStart(2,'0')}`:`${minutes}:${String(seconds%60).padStart(2,'0')}`;
  return `执行中 · ${clock}`;
}

export function activeQuestionIndex(questions, positions, anchor, atBottom, latest) {
  if (!questions.length) return -1;
  if (atBottom && latest) return questions.length - 1;
  const row = atBottom ? positions.at(-1) : positions.filter(row => row.top <= anchor).at(-1) || positions[0];
  return questions.findIndex(question => question.turn === row?.turn);
}

export function computerAddress(value) {
  const url = new URL(value.trim());
  if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash || !['', '/', '/cxpalm', '/cxpalm/'].includes(url.pathname)) {
    throw new Error('请填写电脑的 HTTPS 地址，不要包含账号、查询参数、片段或其他路径。');
  }
  return url.origin;
}

// Keep previously loaded turns, replacing overlapping records with newer data.
export function mergeHistoryTurns(before = [], after = []) {
  const turns = new Map();
  for (const turn of [...before, ...after]) turns.set(turn.turnId || turn.id, turn);
  return [...turns.values()];
}
export function extendHistory(current, page) {
  if (!current) return page;
  const earlier = page.offset < current.offset;
  return {...page, offset: Math.min(page.offset, current.offset),
    hasMore: earlier ? current.hasMore : page.hasMore,
    turns: earlier ? mergeHistoryTurns(page.turns, current.turns) : mergeHistoryTurns(current.turns, page.turns)};
}

// Display attention without replacing official runtime/read metadata.
export function sessionIndicator(status, unread, archived = false) {
  if (archived) return null;
  if (status === 'running' || status === 'waiting') return status;
  if (unread !== true) return null;
  return ['failed','interrupted'].includes(status) ? status : 'completed';
}
export function effortLabel(value) {
  return ({default:'模型默认',minimal:'最少',low:'低',medium:'中',high:'高',xhigh:'超高'})[value] || value;
}

export function offlineReading(snapshot) {
  if(!snapshot)return null;
  const value={...snapshot};
  delete value.requests;delete value.hasUnreadTurn;delete value.readToken;
  value.threadRuntimeStatus={type:'unknown'};
  return value;
}
export class ReadingCache {
  constructor(storage,scope){
    this.storage=storage;this.key=`cxpalm:reading:${scope}`;this.rows=new Map();
    try{const saved=JSON.parse(storage.getItem(this.key)||'null');if(saved?.schema===1&&Date.now()-saved.at>=0&&Date.now()-saved.at<43200000)for(const [id,row] of saved.rows||[])if(typeof id==='string'&&id.length<=128)this.rows.set(id,row);}catch{}
  }
  remember(id,row){
    if(!row.snapshot)return;
    this.rows.delete(id);this.rows.set(id,{...row,snapshot:offlineReading(row.snapshot)});
    while(this.rows.size>8)this.rows.delete(this.rows.keys().next().value);
  }
  get(id){const row=this.rows.get(id);return row?structuredClone(row):null;}
  save(){
    const rows=[...this.rows];
    while(rows.length&&JSON.stringify(rows).length>900000)rows.shift();
    try{this.storage.setItem(this.key,JSON.stringify({schema:1,at:Date.now(),rows}));}catch{}
  }
  clear(){this.rows.clear();try{this.storage.removeItem(this.key);}catch{}}
}

export function seedHistoryIndex(page, snapshot, indexOnly) {
  return indexOnly ? {...page, turns:snapshot?.turns||[]} : page;
}
