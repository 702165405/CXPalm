const fs = require('node:fs');
const path = require('node:path');
const {randomUUID,createHash} = require('node:crypto');
const {execFile} = require('node:child_process');

function readJSON(file) {
  try {
    const raw=fs.readFileSync(file,'utf8');
    if(Buffer.byteLength(raw)>65536)throw new Error('配置文件过大。');
    const value=JSON.parse(raw);
    if(!value||typeof value!=='object'||Array.isArray(value))throw new Error('配置文件必须是 JSON 对象。');
    return {raw,value};
  } catch(error) {if(error.code==='ENOENT')return {raw:null,value:{}};throw error;}
}
function atomicJSON(file,value) {
  fs.mkdirSync(path.dirname(file),{recursive:true,mode:0o700});
  const temporary=file+'.'+randomUUID()+'.tmp';
  try {fs.writeFileSync(temporary,JSON.stringify(value,null,2)+'\n',{flag:'wx',mode:0o600});fs.renameSync(temporary,file);}
  finally {try{fs.unlinkSync(temporary);}catch(error){if(error.code!=='ENOENT')throw error;}}
}
function connection(file,home,codexHome) {
  const {raw,value}=readJSON(file);
  return {value:raw===null?{listen:'127.0.0.1:18789',origin:'',dataDir:path.join(path.dirname(file),'data'),codexHome:codexHome||path.join(home,'.codex'),ipcPath:''}:value,
    revision:createHash('sha256').update(file+'\0'+(raw??'missing')).digest('hex')};
}
function editConnection(current,input) {
  const keys=['revision','origin','sshEnabled','sshHost','sshUser','sshPort','sshRemotePort'];
  if(!input||typeof input!=='object'||Array.isArray(input)||Object.keys(input).length!==keys.length||Object.keys(input).some(key=>!keys.includes(key))||
    typeof input.revision!=='string'||typeof input.origin!=='string'||input.origin.length>2048||typeof input.sshEnabled!=='boolean'||
    typeof input.sshHost!=='string'||input.sshHost.length>253||typeof input.sshUser!=='string'||input.sshUser.length>64||
    !Number.isInteger(input.sshPort)||!Number.isInteger(input.sshRemotePort))throw new Error('连接设置格式无效。');
  if(input.revision!==current.revision)throw new Error('配置已被其他程序修改，请重新读取后再保存。');
  if(input.sshEnabled&&(!input.sshHost.trim()||input.sshRemotePort<1024||input.sshRemotePort>65535||input.sshPort<0||input.sshPort>65535))throw new Error('请填写服务器地址和有效端口，远程转发端口范围为 1024–65535。');
  return {...current.value,origin:input.origin.trim().replace(/\/$/,''),sshHost:input.sshEnabled?input.sshHost.trim():'',sshUser:input.sshEnabled?input.sshUser.trim():'',sshPort:input.sshEnabled?input.sshPort:0,sshRemotePort:input.sshEnabled?input.sshRemotePort:0};
}
function validateConfig(binary,value) {
  return new Promise((resolve,reject)=>{
    const child=execFile(binary,['validate-config'],{windowsHide:true,shell:false,timeout:5000,maxBuffer:8192},(error,_stdout,stderr)=>{
      if(error)reject(new Error('配置校验失败：'+(stderr.trim()||'请检查内置 Agent 和连接设置。')));else resolve();
    });
    child.stdin.on('error',()=>{});child.stdin.end(JSON.stringify(value));
  });
}
function loginOptions(app,enabled) {
  return {openAtLogin:enabled,path:app.getPath('exe'),args:app.isPackaged?['--login-start']:[app.getAppPath(),'--login-start']};
}
function shouldAutoStart(loginLaunch,loginEnabled,appEnabled) {
  return loginLaunch ? loginEnabled===true : appEnabled===true;
}
function validateStartup(input) {
  if(!input||typeof input!=='object'||Array.isArray(input)||Object.keys(input).length!==2||typeof input.loginAutoStart!=='boolean'||typeof input.appAutoStart!=='boolean')throw new Error('启动设置格式无效。');
}
function saveStartup(app,file,input) {
  validateStartup(input);
  const previous=app.getLoginItemSettings(loginOptions(app,true)).openAtLogin;
  const changed=previous!==input.loginAutoStart;
  if(changed) {
    app.setLoginItemSettings(loginOptions(app,input.loginAutoStart));
    if(app.getLoginItemSettings(loginOptions(app,true)).openAtLogin!==input.loginAutoStart) {
      app.setLoginItemSettings(loginOptions(app,previous));
      throw new Error('系统未接受登录启动设置，请检查系统启动项权限。');
    }
  }
  try {atomicJSON(file,{...readJSON(file).value,appAutoStart:input.appAutoStart});}
  catch(error) {if(changed)app.setLoginItemSettings(loginOptions(app,previous));throw error;}
}
module.exports={readJSON,atomicJSON,connection,editConnection,validateConfig,loginOptions,shouldAutoStart,validateStartup,saveStartup};
