const test=require('node:test');
const assert=require('node:assert/strict');
const {inspect}=require('./check-repository.cjs');
const check=(file,text='')=>inspect(file,Buffer.from(text),'/fixture/home');

test('blocks runtime state even if it was already tracked',()=>{
  for(const file of ['.local/account.json','nested/node_modules/a.js','AGENTS.md','cxpalm.json','android/local.properties','data/store.sqlite-wal','.aws/config','app.apk','.env.production','logs/crash.txt']) {
    assert.ok(check(file).some(row=>row.rule==='private-or-generated-file'),file);
  }
});
test('permits source, lockfiles, licenses, assets and fabricated examples',()=>{
  for(const file of ['go.sum','desktop/package-lock.json','web/marked-vendor.mjs','assets/icon.png','deploy/cxpalm.example.json','SECURITY.md','.env.example']) assert.deepEqual(check(file),[]);
});
test('flags selected credentials without returning the matching value',()=>{
  const secrets=['-----BEGIN '+'PRIVATE KEY-----','gh'+'p_'+'A'.repeat(36),'github_'+'pat_'+'a'.repeat(48),'s'+'k-proj-'+'a'.repeat(48),'AK'+'IA'+'A'.repeat(16),'ey'+'J'+'a'.repeat(20)+'.'+'b'.repeat(24)+'.'+'c'.repeat(24)];
  for(const secret of secrets) {
    const result=check('source.js','line one\n'+secret);
    assert.equal(result.length,1);
    assert.equal(result[0].line,2);
    assert.ok(!JSON.stringify(result).includes(secret));
  }
});
test('flags the current machine home but not runtime path construction',()=>{
  assert.equal(check('doc.md','/fixture/home/private/config')[0].rule,'local-home-path');
  assert.equal(inspect('doc.md',Buffer.from(String.raw`C:\\Users\\local-person\\config`),String.raw`C:\Users\local-person`)[0].rule,'local-home-path');
  assert.deepEqual(check('source.js',"path.join(os.homedir(),'.codex')"),[]);
});
test('unknown binary content requires manual review',()=>{
  assert.equal(inspect('unknown.bin',Buffer.from([0,1,2]))[0].rule,'unexpected-binary-manual-review');
  assert.deepEqual(inspect('assets/icon.png',Buffer.from([0,1,2])),[]);
});
