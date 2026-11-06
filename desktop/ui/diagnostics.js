(function(root,factory){
  if(typeof module==='object'&&module.exports)module.exports=factory();else root.CXPalmDiagnostics=factory();
})(globalThis,()=>{
  function view(state={},now=Date.now()){
    const configured=!!state.origin,owned=state.owned===true&&state.state==='running';
    const checkedAt=Number.isSafeInteger(state.diagnosedAt)&&state.diagnosedAt>0&&state.diagnosedAt<=now?state.diagnosedAt:null;
    const fresh=checkedAt!==null&&now-checkedAt<5*60*1000;
    const local={id:'local',label:'本机网关 / 监听',value:owned?'本窗口网关正在监听':state.state==='occupied'?'端口被其他进程占用':configured?'网关未启动':'尚未载入配置',tone:owned?'good':configured?'warn':'',detail:owned?'本机监听由本窗口管理；这不代表公网已连通。':state.state==='occupied'?'未接管外部进程；请先确认占用来源。':configured?'请在总览启动网关，再连接手机或 Web。':'先保存本机连接设置。',action:configured?'overview':'settings'};
    const desktop={id:'desktop',label:'官方 Codex 桌面',value:!checkedAt?'尚未检测':!fresh?'上次检测已过期':state.desktopConnected===true?'最近检测可连接':state.desktopConnected===false?'检测时不可连接':'尚未检测',tone:fresh&&state.desktopConnected===true?'good':checkedAt?'warn':'',detail:!checkedAt?'点击检查连接，只验证本机官方桌面 IPC。':!fresh?'旧结果不能代表当前状态，请重新检测。':state.desktopConnected===true?'仅验证官方桌面 IPC 握手，不代表任务执行或公网可达。':'请打开官方 Codex，再重新检测。',action:'diagnose'};
    const tunnel={id:'tunnel',label:'SSH 反向隧道',value:state.sshConfigured?'已配置 · 连通性未验证':'未启用 SSH 隧道',tone:'',detail:state.sshConfigured?'本页没有隧道连通性证据；请核对 SSH 登录、主机指纹与转发权限。':'未配置 SSH；如使用其他入口，请自行确认链路。',action:'settings'};
    const entry={id:'public',label:'公网 HTTPS 入口',value:configured?'已配置 · 未探测':'尚未配置 HTTPS 地址',tone:'',detail:configured?'打开网页可辅助人工检查；页面可打开不代表已连通本机网关。':'先保存本机连接设置。',action:configured?'open-web':'settings'};
    const guidance=!owned?'网关未由本窗口运行；手机和 Web 访问可能不可用，官方任务不受影响。':fresh&&state.desktopConnected===true?'本次仅做只读检查；未发送消息，也未修改会话或服务器。':'官方桌面连接尚未确认时，请先只读查看已有内容；不要重复提交结果未知的操作。';
    return {rows:[local,desktop,tunnel,entry],checkedAt,stale:checkedAt!==null&&!fresh,guidance};
  }
  return {view};
});
