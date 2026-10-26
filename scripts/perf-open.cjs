// Isolated local HTTPS gateway only. No chat mutations, no user content logged.
const {chromium}=require('playwright');
const fs=require('node:fs/promises');
(async()=>{
 const input=JSON.parse(process.env.CXPALM_PERF_INPUT);
 if(!/^https:\/\/127\.0\.0\.1:\d+$/.test(input.url))throw Error('local test gateway required');
 const browser=await chromium.launch({headless:true,executablePath:process.env.CXPALM_TEST_BROWSER});
 try{
  const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:960}});
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(()=>{
   window.__firstReply={};window.__openStarted={};window.__mutations=[];
   const Native=window.WebSocket;
   window.WebSocket=class extends Native{
    constructor(...args){super(...args);this.addEventListener('message',event=>{const e=JSON.parse(event.data);if(e.type==='chat.state'&&!window.__firstReply[e.data.id]&&e.data.snapshot?.turns?.some(t=>t.items?.some(i=>i.type==='agentMessage'&&i.text)))window.__firstReply[e.data.id]=performance.now();});}
    send(raw){const q=JSON.parse(raw);if(q.method==='chat.subscribe'){delete window.__firstReply[q.thread];window.__openStarted[q.thread]=performance.now();}else if(!['sessions.browse','sessions.subscribe','notifications.subscribe','chat.catalog','operation.get'].includes(q.method))window.__mutations.push(q.method);super.send(raw);}
   };
  });
  await page.goto(input.url+'/cxpalm/');
  await page.locator('#login-username').fill(input.username);await page.locator('#login-password').fill(input.password);await page.locator('#login-submit').click();await page.locator('.session-item').first().waitFor();
  const times=[],paint=[];
  for(let pass=0;pass<3;pass++)for(const id of input.ids){
   // Use stable thread IDs, not title text or potentially reordered project groups.
   const button=page.locator(`.session-item[data-thread="${id}"]`);
   await button.click();const start=await page.evaluate(id=>window.__openStarted[id],id);
   await page.waitForFunction(id=>window.__firstReply[id]>0,id,{timeout:6000});
   await page.waitForFunction(()=>Array.from(document.querySelectorAll('#messages .message')).some(n=>n.querySelector('.message-head')?.textContent==='Codex'&&n.querySelector('.message-body')?.textContent.trim()));
   const frame=await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>resolve(performance.now()))));
   const first=await page.evaluate(id=>window.__firstReply[id],id);times.push(first-start);paint.push(frame-start);
  }
  if(errors.length)throw Error(errors.join(';'));if((await page.evaluate(()=>window.__mutations)).length)throw Error('unexpected write in profile');
  await fs.writeFile(input.output,JSON.stringify({firstBodyMs:times,paintMs:paint,samples:times.length,transport:'real local HTTPS/WebSocket',writes:0},null,2));
  console.log('15 real local HTTPS/WebSocket click-to-reply samples, no mutations or content logged');
 }finally{await browser.close();}
})().catch(e=>{console.error(e.message);process.exitCode=1});
