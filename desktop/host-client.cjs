const {spawn} = require('node:child_process');
const {EventEmitter} = require('node:events');
const {randomUUID} = require('node:crypto');
const methods = new Set(['status','start','stop','devices','devices.revoke','device.logs','ip-policy.get','ip-policy.set','login.get','login.set','diagnose','shutdown']);
function validate(method, args = {}) {
  if (!methods.has(method) || !args || typeof args !== 'object' || Array.isArray(args)) throw new Error('不支持的桌面操作。');
  if(method==='login.set'){
    const v=args.login;
    if(Object.keys(args).length!==1||!v||typeof v!=='object'||Array.isArray(v)||Object.keys(v).some(key=>!['username','password','expected'].includes(key))||typeof v.username!=='string'||!v.username.trim()||Buffer.byteLength(v.username)>80||typeof v.password!=='string'||[...v.password].length<12||Buffer.byteLength(v.password)>256||typeof v.expected!=='string'||!/^([a-f0-9]{64})?$/.test(v.expected))throw new Error('账号或密码格式无效。密码至少 12 个字符。');
    return;
  }
  if(method==='ip-policy.set'){
    const p=args.policy;
    if(Object.keys(args).length!==1||!p||typeof p!=='object'||Array.isArray(p)||Object.keys(p).some(key=>!['enabled','entries'].includes(key))||typeof p.enabled!=='boolean'||!Array.isArray(p.entries)||p.entries.length>64||p.entries.some(value=>typeof value!=='string'||!value.trim()||value.length>80))throw new Error('白名单格式无效。');
    if(p.enabled&&!p.entries.length)throw new Error('开启白名单前至少填写一个 IP 或网段。');
    return;
  }
  if (Object.keys(args).some(key => key !== 'device')) throw new Error('不接受路径、命令或额外参数。');
  if (method === 'devices.revoke' ? !/^[a-f0-9]{32}$/.test(args.device || '') : Object.keys(args).length) throw new Error('设备参数无效。');
}
class HostClient extends EventEmitter {
  constructor(binary, config, spawnProcess = spawn, record = () => {}) {
    super(); this.pending = new Map(); this.buffer = ''; this.closed = false;
    this.record=record;
    this.child = spawnProcess(binary, ['desktop-host','--config',config], {stdio:['pipe','pipe','pipe'],windowsHide:true,shell:false});
    this.child.stdout.setEncoding('utf8');
    this.child.stdout.on('data', chunk => this.read(chunk));
    // Never forward stdout/stderr: account requests and responses may contain credentials.
    this.child.stderr.resume();
    this.child.on('error', () => {this.record('host-spawn-error');this.fail('桌面管理进程无法启动，请检查 Agent 文件。');});
    this.child.on('exit', (code,signal) => {this.record('host-exit',{code,signal});this.fail('桌面管理连接已关闭；本窗口启动的 Agent 将停止。');});
    this.child.stdin.on('error', () => {this.record('host-pipe-error');this.fail('桌面管理管道已断开。');});
  }
  read(chunk) {
    this.buffer += chunk;
    if (this.buffer.length > 2 * 1024 * 1024) {this.record('host-protocol-error');return this.fail('桌面管理响应超过大小限制。');}
    for (let at; (at = this.buffer.indexOf('\n')) >= 0;) {
      const line = this.buffer.slice(0, at); this.buffer = this.buffer.slice(at + 1);
      let reply; try { reply = JSON.parse(line); } catch { this.record('host-protocol-error');return this.fail('桌面管理响应格式无效。'); }
      if (!reply || typeof reply !== 'object' || Array.isArray(reply) || typeof reply.id !== 'string') {this.record('host-protocol-error');return this.fail('桌面管理响应格式无效。');}
      const pending = this.pending.get(reply.id); if (!pending) continue;
      this.pending.delete(reply.id); clearTimeout(pending.timer);
      if (reply.error) pending.reject(new Error(reply.error)); else pending.resolve(reply.data);
    }
  }
  request(method, args = {}) {
    try { validate(method,args); } catch (error) { return Promise.reject(error); }
    if (this.closed) return Promise.reject(new Error('桌面管理连接未建立。'));
    if (this.pending.size >= 16) return Promise.reject(new Error('操作正在处理中，请稍候。'));
    const id = randomUUID();
    return new Promise((resolve,reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); this.record('host-timeout',{method});reject(new Error('操作结果未确认。请刷新状态，不要重复提交。')); }, 12000);
      this.pending.set(id,{resolve,reject,timer});
      this.child.stdin.write(JSON.stringify({id,method,...args})+'\n');
    });
  }
  fail(message) {
    if (this.closed) return; this.closed = true;
    this.record('host-closed',{pending:this.pending.size});
    for (const item of this.pending.values()) {clearTimeout(item.timer);item.reject(new Error(message));}
    this.pending.clear(); this.child.stdin.end(); this.emit('closed');
  }
  async close() {
    this.record('host-close-request');
    try { if (!this.closed) await this.request('shutdown'); }
    finally { this.child.stdin.end(); this.closed = true; }
  }
}
module.exports = {HostClient,validate};
