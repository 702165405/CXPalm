const fs=require('node:fs');
const path=require('node:path');
const {randomUUID}=require('node:crypto');

const events=new Set(['session-start','heartbeat','previous-session-unfinished','process-exit','main-exception','startup-failed','startup-ready','config-detected','host-connected','host-spawn-error','host-exit','host-pipe-error','host-protocol-error','host-timeout','host-close-request','host-closed','agent-action','renderer-gone','child-gone','window-unresponsive','window-responsive','window-hidden','window-shown','window-destroyed','quit-request','quit-blocked','quit-cancelled','quit-confirmed','quit-failed','will-quit','system-session-end','update-install-confirmed']);
const reasons=new Set(['tray','application','update','test','settings-busy','clean-exit','abnormal-exit','crashed','oom','killed','integrity-failure','launch-failed','normal-exit','still-running','memory-eviction','uncaughtException','unhandledRejection']);
const types=new Set(['Browser','GPU','Utility','Renderer','Zygote','Sandbox helper','Unknown']);
const errorTypes=new Set(['Error','TypeError','RangeError','SyntaxError','ReferenceError','URIError','EvalError','AggregateError']);
const methods=new Set(['status','start','stop','diagnose','devices','devices.revoke','device.logs','ip-policy.get','ip-policy.set','login.get','login.set','shutdown']);

// Only fixed labels, bounded numbers and booleans reach disk. Never serialize
// errors, command lines, config paths, IPC payloads, stdout or stderr.
function fields(input={}) {
  const result={};
  for(const key of ['pid','code','pending'])if(Number.isSafeInteger(input[key]))result[key]=input[key];
  for(const key of ['success','configured','owned','loginLaunch','clean','visible'])if(typeof input[key]==='boolean')result[key]=input[key];
  if(reasons.has(input.reason))result.reason=input.reason;
  if(types.has(input.type))result.type=input.type;
  if(errorTypes.has(input.errorType))result.errorType=input.errorType;
  if(methods.has(input.method))result.method=input.method;
  if(typeof input.version==='string'&&/^\d{1,4}\.\d{1,4}\.\d{1,4}$/.test(input.version))result.version=input.version;
  if(typeof input.signal==='string'&&/^SIG[A-Z]{1,12}$/.test(input.signal))result.signal=input.signal;
  if(Array.isArray(input.locations))result.locations=input.locations.filter(value=>typeof value==='string'&&/^(?:main|host-client|settings|updater|diagnostics)\.cjs:\d{1,7}:\d{1,7}$/.test(value)).slice(0,6);
  return result;
}

function createDiagnostics(directory,{version,loginLaunch=false,maxBytes=256*1024}={}) {
  const file=path.join(directory,'lifecycle.jsonl'),marker=path.join(directory,'session.json');
  const session=randomUUID();let finished=false;
  function record(event,input) {
    if(!events.has(event))return;
    try {
      fs.mkdirSync(directory,{recursive:true,mode:0o700});
      const line=JSON.stringify({time:new Date().toISOString(),session,event,...fields(input)})+'\n';
      if(fs.existsSync(file)&&fs.statSync(file).size+Buffer.byteLength(line)>maxBytes) {
        if(fs.existsSync(file+'.2'))fs.unlinkSync(file+'.2');
        if(fs.existsSync(file+'.1'))fs.renameSync(file+'.1',file+'.2');
        fs.renameSync(file,file+'.1');
      }
      fs.appendFileSync(file,line,{mode:0o600});
    } catch {} // A full/unwritable disk must not stop the app or its gateway.
  }
  function state(clean) {
    const temporary=marker+'.'+session+'.tmp';
    try {
      fs.mkdirSync(directory,{recursive:true,mode:0o700});
      fs.writeFileSync(temporary,JSON.stringify({session,pid:process.pid,clean}),{mode:0o600});
      fs.renameSync(temporary,marker);
    } catch {} finally {try{fs.unlinkSync(temporary);}catch{}}
  }
  try {
    const stat=fs.statSync(marker);
    if(stat.size<=4096) {
      const previous=JSON.parse(fs.readFileSync(marker,'utf8'));
      if(previous.clean===false)record('previous-session-unfinished',{pid:previous.pid});
    }
  } catch {}
  state(false);record('session-start',{version,loginLaunch,pid:process.pid});
  return {record,exception(error,origin) {
    const locations=typeof error?.stack==='string'?error.stack.split('\n').filter(line=>/^\s+at /.test(line)).map(line=>line.match(/\b((?:main|host-client|settings|updater|diagnostics)\.cjs:\d{1,7}:\d{1,7})\)?$/)?.[1]).filter(Boolean):[];
    record('main-exception',{errorType:error?.name,reason:origin,locations});
  },finish(code,clean) {
    if(finished)return;finished=true;
    record('process-exit',{code,clean});
    if(clean===true)state(true);
  }};
}
module.exports={createDiagnostics};
