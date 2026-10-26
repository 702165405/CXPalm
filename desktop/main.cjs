const {app,BrowserWindow,ipcMain,dialog,Tray,Menu,nativeImage,shell,clipboard} = require('electron');
const fs = require('node:fs');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const {HostClient,validate} = require('./host-client.cjs');
const settings = require('./settings.cjs');
const {create,choose,valid}=require('./ui/i18n.js');
const messages=JSON.parse(fs.readFileSync(app.isPackaged?path.join(process.resourcesPath,'locales/en.json'):path.join(__dirname,'../locales/en.json'),'utf8'));
let languageUI;
const tr=(value,...args)=>languageUI?languageUI.tr(value,...args):value;
const {DesktopUpdater}=require('./updater.cjs');
const fixture = !app.isPackaged && process.env.CXPALM_DESKTOP_FIXTURE === '1';
const smoke = fixture && process.env.CXPALM_DESKTOP_SMOKE === '1';
let updater, reconnecting;
let win, tray, client, configPath = '', lastState = null, quitting = false, confirmingQuit = false, connectionError = '';
let settingsBusy=false, startupError='', fixtureLogin=fixture&&process.env.CXPALM_DESKTOP_FIXTURE_LOGIN==='1';
const loginLaunch=process.argv.includes('--login-start');
const loginApp=fixture?{isPackaged:true,getPath:kind=>app.getPath(kind),getLoginItemSettings:()=>({openAtLogin:fixtureLogin}),setLoginItemSettings:options=>{fixtureLogin=options.openAtLogin;}}:app;
const pageURL = pathToFileURL(path.join(__dirname,'ui/index.html')).href;
const hasLock=app.requestSingleInstanceLock();
const diagnostics=hasLock?require('./diagnostics.cjs').createDiagnostics(path.join(app.getPath('userData'),'logs'),{version:app.isPackaged?app.getVersion():require('./package.json').version,loginLaunch}):null;
const record=(event,details)=>diagnostics?.record(event,details);
let quitReason='application',cleanExit=false;
if(hasLock){
  process.on('uncaughtExceptionMonitor',(error,origin)=>diagnostics.exception(error,origin));
  process.on('exit',code=>diagnostics.finish(code,cleanExit&&code===0));
  app.on('will-quit',()=>{cleanExit=true;record('will-quit',{reason:quitReason});});
  app.on('child-process-gone',(_event,details)=>record('child-gone',{type:details.type,reason:details.reason,code:details.exitCode}));
  setInterval(()=>record('heartbeat',{configured:!!configPath,owned:lastState?.owned===true}),60000).unref();
}
if (!hasLock) {app.quit();} else {
  app.on('second-instance',()=>{if(win){win.show();win.focus();}});
}
function trusted(event) {if (!win || event.sender !== win.webContents || event.senderFrame !== win.webContents.mainFrame || event.senderFrame.url !== pageURL) throw new Error('拒绝非本地窗口请求。');}
function preferencesPath(){return path.join(app.getPath('userData'),'desktop-preferences.json');}
function detectConfig(){
  const candidates = [];
  try {candidates.push(JSON.parse(fs.readFileSync(preferencesPath(),'utf8')).configPath);} catch {}
  const at = process.argv.indexOf('--config'); if(at>=0)candidates.unshift(process.argv[at+1]);
  candidates.push(path.join(path.dirname(app.getPath('exe')),'cxpalm.json'),path.join(app.getPath('userData'),'cxpalm.json'));
  if (!app.isPackaged) candidates.push(path.join(__dirname,'../.local/cxpalm.json'));
  return candidates.find(value=>typeof value==='string'&&path.isAbsolute(value)&&fs.existsSync(value)) || '';
}
function preferences(){return settings.readJSON(preferencesPath()).value;}
function connectionFile(){return configPath||path.join(app.getPath('userData'),'cxpalm.json');}
function readConnection(){return settings.connection(connectionFile(),app.getPath('home'),process.env.CODEX_HOME);}
function getSettings(){
  const {value,revision}=readConnection();
  return {configPath:connectionFile(),revision,origin:value.origin||'',sshEnabled:!!value.sshHost,sshHost:value.sshHost||'',sshUser:value.sshUser||'',sshPort:value.sshPort||0,sshRemotePort:value.sshRemotePort||18791,
    loginAutoStart:loginApp.getLoginItemSettings(settings.loginOptions(loginApp,true)).openAtLogin,appAutoStart:preferences().appAutoStart===true,startMinimized:preferences().startMinimized===true,startupSupported:['win32','darwin'].includes(process.platform)};
}
async function lockedSettings(operation){
  if(settingsBusy)throw new Error('设置正在保存，请稍后再试。');
  settingsBusy=true;try{return await operation();}finally{settingsBusy=false;}
}
function binaryPath(){return app.isPackaged ? path.join(process.resourcesPath,'agent',process.platform==='win32'?'cxpalm.exe':'cxpalm') : path.join(__dirname,'../.local/desktop-build/agent',process.platform==='win32'?'cxpalm.exe':'cxpalm');}
async function connect(file){
  if(client)await client.close();
  client=null;lastState=null;connectionError='';configPath=file;
  if(fixture){client=require('./test/fixture.cjs').create();}
  else client=new HostClient(binaryPath(),file,undefined,record);
  const connectedClient=client;
  client.on('closed',()=>{if(client!==connectedClient)return;lastState=null;connectionError='桌面管理连接已断开，请刷新状态恢复连接。';updateTray();});
  try{const state=await client.request('status');record('host-connected');return snapshot(state);}catch(error){await client.close().catch(()=>{});client=null;connectionError='无法载入配置或启动桌面管理进程。请检查所选 JSON 和内置 Agent。';throw error;}
}
async function ensureConnection(){
  if(reconnecting){await reconnecting;return !!client&&!client.closed;}
  if(client&&!client.closed)return true;
  if(!configPath)return false;
  // Reconnect only before dispatch. Never replay a command whose outcome is unknown.
  if(!reconnecting)reconnecting=connect(configPath).finally(()=>{reconnecting=null;});
  await reconnecting;
  return !!client&&!client.closed;
}
function snapshot(state){lastState=state;updateTray();return {...state,configPath,fixture,startupError};}
function updateTray(){
  if(!tray)return;
  const owned=lastState?.owned;
  tray.setToolTip(`CXPalm · ${owned?tr('Agent 运行中'):tr('桌面控制台')}`);
  tray.setContextMenu(Menu.buildFromTemplate([
    {label:tr('打开 CXPalm'),click:()=>{win.show();win.focus();}},
    {type:'separator'},
    {label:tr('启动 Agent'),enabled:!!client&&!owned&&lastState?.state!=='occupied',click:()=>trayAction('start')},
    {label:tr('停止本窗口的 Agent'),enabled:!!owned,click:()=>trayAction('stop')},
    {type:'separator'},{label:tr('退出 CXPalm'),click:()=>{quitReason='tray';app.quit();}},
  ]));
}
async function trayAction(method){
  try{if(settingsBusy)throw new Error('设置正在保存');snapshot(await client.request(method));record('agent-action',{method,success:true});win.webContents.send('desktop:changed');}
  catch{record('agent-action',{method,success:false});dialog.showErrorBox('CXPalm',tr('操作未完成。请打开控制台刷新状态；不要重复提交。'));}
}
const ready=hasLock?app.whenReady().then(async()=>{
  languageUI=create(messages,preferences().language||(fixture?'zh-CN':choose(app.getLocale())));
  app.setAppUserModelId('dev.cxpalm.desktop');
  const updaterOptions={releaseLine:fixture?null:'fresh-001',version:fixture?require('./package.json').version:app.getVersion(),cacheDir:path.join(app.getPath('userData'),'updates'),packaged:app.isPackaged};
  updater=new DesktopUpdater(fixture?{...updaterOptions,packaged:true,...require('./test/fixture.cjs').updateOptions()}:updaterOptions);
  updater.on('changed',value=>{if(win&&!win.isDestroyed())win.webContents.send('desktop:update-changed',value);});
  Menu.setApplicationMenu(process.platform==='darwin'?Menu.buildFromTemplate([{role:'appMenu'},{role:'editMenu'},{role:'windowMenu'}]):null);
  const icon=app.isPackaged?path.join(process.resourcesPath,'icon.png'):path.join(__dirname,'../assets/icon.png');
  win=new BrowserWindow({width:1280,height:860,minWidth:900,minHeight:650,show:false,title:'CXPalm',backgroundColor:'#f6f7fa',icon,
    titleBarStyle:'hidden',titleBarOverlay:process.platform==='win32'?{color:'#ffffff',symbolColor:'#1b2230',height:38}:undefined,
    webPreferences:{offscreen:smoke,backgroundThrottling:!smoke,preload:path.join(__dirname,'preload.cjs'),sandbox:true,contextIsolation:true,nodeIntegration:false,webSecurity:true,devTools:!app.isPackaged}});
  win.webContents.setWindowOpenHandler(()=>({action:'deny'}));
  win.webContents.on('will-navigate',event=>event.preventDefault());
  win.webContents.on('render-process-gone',(_event,details)=>record('renderer-gone',{reason:details.reason,code:details.exitCode}));
  win.on('unresponsive',()=>record('window-unresponsive'));
  win.on('responsive',()=>record('window-responsive'));
  win.on('hide',()=>record('window-hidden'));
  win.on('show',()=>record('window-shown'));
  win.on('closed',()=>record('window-destroyed'));
  win.on('query-session-end',()=>record('system-session-end'));
  win.on('session-end',()=>record('system-session-end'));
  win.webContents.session.setPermissionRequestHandler((_contents,_permission,callback)=>callback(false));
  win.webContents.session.setPermissionCheckHandler(()=>false);
  win.webContents.session.on('will-download',event=>event.preventDefault());
  win.on('close',event=>{if(!quitting){event.preventDefault();win.hide();}});
  tray=new Tray(nativeImage.createFromPath(icon).resize({width:20,height:20}));tray.on('click',()=>{win.show();win.focus();});updateTray();
  ipcMain.handle('desktop:invoke',async(event,{method,args}={})=>{
    trusted(event);validate(method,args);
    if(settingsBusy)throw new Error('设置正在保存，请稍后再试。');
    if(method==='shutdown')throw new Error('请通过托盘退出桌面端。');
    if(!await ensureConnection()){if(method!=='status')throw new Error('请先选择有效的 CXPalm 配置。');return {state:connectionError?'error':'unconfigured',fixture,configPath,error:connectionError,startupError};}
    let data;
    try{data=await client.request(method,args);if(['start','stop'].includes(method))record('agent-action',{method,success:true});}
    catch(error){if(['start','stop'].includes(method))record('agent-action',{method,success:false});throw error;}
    if(['status','start','stop','diagnose'].includes(method))return snapshot(data);
    return data;
  });
  ipcMain.handle('desktop:choose-config',async event=>{
    trusted(event);
    return lockedSettings(async()=>{
    if(client&&!client.closed) snapshot(await client.request('status'));
    if(lastState?.owned)throw new Error('切换配置前请先停止本窗口的 Agent。');
    const result=await dialog.showOpenDialog(win,{title:tr('选择 CXPalm 私有配置'),properties:['openFile'],filters:[{name:tr('CXPalm 配置'),extensions:['json']}]});
    if(result.canceled)return null;
    const resultState=await connect(result.filePaths[0]);
    settings.atomicJSON(preferencesPath(),{...preferences(),configPath});
    return resultState;
    });
  });
  ipcMain.handle('desktop:language-get',event=>{trusted(event);return {language:languageUI.code,messages};});
  ipcMain.handle('desktop:language-set',(event,value)=>{
    trusted(event);if(!valid(value))throw new Error('语言选择无效。');
    return lockedSettings(async()=>{
      settings.atomicJSON(preferencesPath(),{...preferences(),language:value});
      languageUI.set(value);updateTray();return {language:languageUI.code};
    });
  });
  ipcMain.handle('desktop:settings-get',event=>{trusted(event);return getSettings();});
  ipcMain.handle('desktop:settings-connection',async(event,input)=>{
    trusted(event);return lockedSettings(async()=>{
      if(client&&!client.closed)snapshot(await client.request('status'));
      if(lastState?.owned)throw new Error('保存连接设置前，请先停止本窗口的网关。启动设置可单独保存。');
      const file=connectionFile(),current=readConnection(),value=settings.editConnection(current,input);
      await settings.validateConfig(binaryPath(),value);
      // Recheck after async validation so an external edit cannot be overwritten.
      if(current.revision!==readConnection().revision)throw new Error('配置已被其他程序修改，请重新读取后再保存。');
      const prefs=preferences();
      settings.atomicJSON(file,value);
      settings.atomicJSON(preferencesPath(),{...prefs,configPath:file});
      let result;
      try{result=await connect(file);}catch{result={state:'error',configPath:file,error:connectionError,fixture};}
      return {settings:getSettings(),state:result};
    });
  });
  ipcMain.handle('desktop:settings-startup',async(event,input)=>{
    trusted(event);return lockedSettings(async()=>{
      settings.validateStartup(input);
      if(!['win32','darwin'].includes(process.platform))throw new Error('当前系统不支持登录启动。');
      if(!fixture&&!app.isPackaged&&input?.loginAutoStart)throw new Error('请在打包后的桌面 App 中启用登录启动。');
      if(input?.loginAutoStart||input?.appAutoStart){
        if(!configPath)throw new Error('请先保存有效的连接设置，再启用自动启动。');
        await settings.validateConfig(binaryPath(),readConnection().value);
        settings.atomicJSON(preferencesPath(),{...preferences(),configPath});
      }
      settings.saveStartup(loginApp,preferencesPath(),input);return getSettings();
    });
  });
  ipcMain.handle('desktop:update-status',event=>{trusted(event);return updater.status();});
  ipcMain.handle('desktop:update-check',event=>{trusted(event);if(settingsBusy)throw new Error('设置或安装操作正在进行。');return updater.check();});
  ipcMain.handle('desktop:update-download',event=>{trusted(event);if(settingsBusy)throw new Error('设置或安装操作正在进行。');return updater.download();});
  ipcMain.handle('desktop:update-cancel',event=>{trusted(event);if(settingsBusy)throw new Error('设置或安装操作正在进行。');return updater.cancel();});
  ipcMain.handle('desktop:update-install',event=>{
    trusted(event);return lockedSettings(async()=>{
      if(fixture)throw new Error('隔离测试不会运行安装程序。');
      await updater.installer();
      if(client&&!client.closed)snapshot(await client.request('status'));
      const answer=await dialog.showMessageBox(win,{type:'question',title:tr('安装 CXPalm 更新？'),message:tr('安装桌面版 {0}？',updater.status().release.version),detail:(lastState?.owned?tr('将停止本窗口的网关，手机连接会断开。'):tr('不会停止外部网关或官方 Codex。'))+tr(' 配置和已有设备授权保留；之后打开 Windows 安装向导，由你确认安装。当前测试包未签名，Windows 可能显示安全提示。'),buttons:[tr('取消'),tr('退出并打开安装向导')],defaultId:0,cancelId:0});
      if(answer.response!==1)return updater.status();
      record('update-install-confirmed');
      const file=await updater.installer();
      updater.set({phase:'installing',message:'正在退出并打开安装向导…'});
      try{
        if(client)await client.close();
        const error=await shell.openPath(file);if(error)throw new Error('无法打开安装向导，请重新下载或稍后再试。网关如已停止，请手动启动。');
      }catch(error){
        if(configPath&&client?.closed){try{await connect(configPath);}catch{}}
        updater.set({phase:'ready',message:error.message});throw error;
      }
      quitReason='update';quitting=true;tray?.destroy();win?.destroy();app.quit();return updater.status();
    });
  });
  ipcMain.handle('desktop:open-web',async event=>{
    trusted(event);if(!lastState?.origin)throw new Error('请先选择配置。');
    const url=new URL(lastState.origin);if(url.protocol!=='https:'||url.username||url.password)throw new Error('地址无效。');
    if(!fixture)await shell.openExternal(new URL('/cxpalm/',url).href);
  });
  ipcMain.handle('desktop:copy',async(event,kind)=>{
    trusted(event);const value=kind==='address'&&lastState?.origin?lastState.origin+'/cxpalm/':null;
    if(!value)throw new Error('没有可复制的内容。');clipboard.writeText(value);
  });
  win.on('show',()=>{if(!fixture&&!settingsBusy)void updater.check(true).catch(()=>{});});
  await win.loadURL(pageURL);
  const loggedIn=loginLaunch||(process.platform==='darwin'&&app.getLoginItemSettings().wasOpenedAtLogin);
  configPath=fixture?(process.env.CXPALM_DESKTOP_FIRST_RUN==='1'?'':path.join(app.getPath('userData'),'cxpalm.json')):detectConfig();
  record('config-detected',{configured:!!configPath});
  if(fixture&&configPath)settings.atomicJSON(configPath,{listen:'127.0.0.1:18789',origin:'https://cxpalm.example.test',dataDir:path.join(app.getPath('userData'),'data'),codexHome:app.getPath('userData'),sshHost:'fixture-host',sshRemotePort:18791});
  if(configPath){try{await connect(configPath);}catch{client=null;}}
  try{
    const start=settings.shouldAutoStart(loggedIn,loginApp.getLoginItemSettings(settings.loginOptions(loginApp,true)).openAtLogin,preferences().appAutoStart);
    if(start){
      if(!client)throw new Error('尚未载入有效配置，请打开设置检查。');
      if(lastState?.state==='occupied')throw new Error('已有进程占用网关端口，本窗口未接管或停止它。');
      snapshot(await client.request('start'));
      record('agent-action',{method:'start',success:true});
    }
  }catch(error){record('agent-action',{method:'start',success:false});startupError='自动启动网关未完成：'+error.message;}
  // Decide only after configuration and optional Agent startup finish. First run
  // and failures must stay visible; explicitly reopening the app still shows it.
  if(!smoke&&settings.shouldShowOnStartup({loginLaunch:loggedIn,startMinimized:preferences().startMinimized,configured:!!configPath&&!!client,failed:!!startupError||!!connectionError}))win.show();
  win.webContents.send('desktop:changed');
  record('startup-ready',{configured:!!configPath&&!!client,owned:lastState?.owned===true,visible:win.isVisible(),success:!startupError&&!connectionError});
  if(!fixture)void updater.check(true).catch(()=>{});
}).catch(error=>{record('startup-failed',{errorType:error?.name});throw error;}):Promise.resolve();
app.on('window-all-closed',()=>{});
app.on('before-quit',event=>{
  if(quitting||!hasLock)return;event.preventDefault();
  record('quit-request',{reason:quitReason,owned:lastState?.owned===true});
  if(settingsBusy){record('quit-blocked',{reason:'settings-busy'});win.show();dialog.showErrorBox('CXPalm',tr('正在处理设置或安装，请完成后再退出。'));return;}
  if(confirmingQuit)return;confirmingQuit=true;
  (async()=>{
    if(lastState?.owned&&!smoke){const answer=await dialog.showMessageBox(win,{type:'question',buttons:[tr('取消'),tr('停止并退出')],defaultId:0,cancelId:0,title:tr('退出 CXPalm？'),message:tr('退出会停止本窗口启动的 Agent 和隧道。'),detail:tr('手机连接将断开；外部 Agent 和官方 Codex 不受影响。')});if(answer.response!==1){record('quit-cancelled');quitReason='application';confirmingQuit=false;return;}}
    record('quit-confirmed',{reason:quitReason});
    quitting=true;updater?.dispose();
    try{if(client)await client.close();}finally{tray?.destroy();win?.destroy();app.quit();}
  })().catch(()=>{record('quit-failed');quitting=false;confirmingQuit=false;dialog.showErrorBox('CXPalm',tr('退出未完成，请刷新状态后重试。'));});
});
module.exports={ready,getWindow:()=>win,closeForTest:async()=>{quitReason='test';quitting=true;updater?.dispose();if(client)await client.close();tray?.destroy();win?.destroy();app.quit();}};
