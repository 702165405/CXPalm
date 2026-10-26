const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {create,choose,valid}=require('../ui/i18n.js'),messages=require('../../locales/en.json');
test('language allowlist, system default and reversible interpolation',()=>{
 assert.equal(choose('zh-TW'),'zh-CN');assert.equal(choose('en-GB'),'en');assert.equal(choose('fr-FR'),'en');assert.equal(valid('en'),true);assert.equal(valid('zh-CN'),true);assert.equal(valid('../../config'),false);
 const ui=create(messages,'en');assert.equal(ui.tr('设置'),'Settings');assert.equal(ui.tr('应用更新'),'App updates');assert.equal(ui.tr('记住登录 · 30 天内无需重新登录'),'Remember sign-in for 30 days');
 assert.equal(ui.tr('安装桌面版 0.0.10？'),'Install desktop version 0.0.10?');
 ui.set('zh-CN');assert.equal(ui.tr('Settings'),'设置');assert.equal(ui.tr('Install desktop version 0.0.10?'),'安装桌面版 0.0.10？');assert.throws(()=>ui.set('de'));
 const content='<script>keep user text</script> 中文 message';assert.equal(ui.tr(content),content);
 assert.equal(ui.tr('即将连接：{0}\n请确认这是你信任的电脑地址。',content),'即将连接：'+content+'\n请确认这是你信任的电脑地址。');
});
test('shared catalogue has English values and retains numbered substitutions',()=>{
 assert.ok(Object.keys(messages).length>500);
 for(const [zh,en] of Object.entries(messages)){
   assert.equal(typeof en,'string');assert.ok(en.trim(),zh);assert.equal(/[\p{Script=Han}]/u.test(en),false,zh);
   const slots=value=>[...value.matchAll(/\{(\d+)\}/g)].map(match=>match[1]).sort();assert.deepEqual(slots(en),slots(zh),zh);
 }
});
test('desktop and Android explicit UI text keys are covered; user content is not swept',()=>{
 const root=path.resolve(__dirname,'../..');
 const files=['desktop/ui/app.js','android/app/src/main/java/dev/cxpalm/SettingsPage.kt','android/app/src/main/java/dev/cxpalm/PalmApp.kt','android/app/src/main/java/dev/cxpalm/ChatPage.kt','android/app/src/main/java/dev/cxpalm/PalmComponents.kt','android/app/src/main/java/dev/cxpalm/ConnectionService.kt','android/app/src/main/java/dev/cxpalm/UiModels.kt','android/app/src/main/java/dev/cxpalm/TaskTiming.kt','android/app/src/main/java/dev/cxpalm/PendingNotice.kt'];
 for(const file of files){const source=fs.readFileSync(path.join(root,file),'utf8');for(const match of source.matchAll(/tr\((['"])((?:\\.|(?!\1).)*?)\1/gs)){
   const key=match[2].replace(/\\n/g,'\n');if(/[\p{Script=Han}]/u.test(key))assert.ok(Object.hasOwn(messages,key),file+': '+key);
 }}
 const main=fs.readFileSync(path.join(root,'desktop/main.cjs'),'utf8');assert.ok(main.includes('if(!valid(value))'));assert.ok(main.includes('...preferences(),language:value'));
 const android=fs.readFileSync(path.join(root,'android/app/src/main/java/dev/cxpalm/PalmComponents.kt'),'utf8');assert.match(android,/MarkdownMessage\((?:message|it)\.text\)/);assert.ok(android.includes('Text(message.text'));assert.ok(!android.includes('tr(message.text)'));assert.ok(!android.includes('tr(it.text)'));assert.ok(!android.includes('tr(computer.name)'));
 const markdown=fs.readFileSync(path.join(root,'android/app/src/main/java/dev/cxpalm/MarkdownMessage.kt'),'utf8');assert.ok(markdown.includes('parseMarkdown(source)'));assert.ok(!markdown.includes('tr(source)'));assert.ok(!markdown.includes('tr(node.literal)'));
});
