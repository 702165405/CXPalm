// Heuristic check of working-tree Git candidates, not commit history or an index snapshot.
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const {execFileSync}=require('node:child_process');

const privatePath=/(^|\/)(?:\.local|\.tools|\.tmp|\.git|\.codex|\.agents|\.aws|\.android|node_modules|uploads|runtime|logs|data|build|dist|\.gradle|\.kotlin)(\/|$)/i;
const privateFile=/(?:\.(?:log|sqlite(?:3)?(?:-\w+)?|db(?:-\w+)?|jsonl|jks|keystore|pem|key|p12|pfx|crt|cer|der|apk|aab|apks|exe|dmg|zip|tar|gz|7z|bak|backup|hprof|credentials)|\.mobileprovision)$/i;
const patterns=[
  ['private-key', /-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----/],
  ['github-token', /\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})\b/],
  ['api-key', /\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{32,}\b/],
  ['aws-access-key', /\b(?:AKIA|ASIA)[A-Z0-9]{16}\b/],
  ['deployment-domain', /\b(?:win|mac)\.lqilt\x2etop\b/i],
  ['jwt', /\beyJ[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\b/],
];

function inspect(file,bytes,home=os.homedir()) {
  const findings=[];
  const name=file.replaceAll('\\','/');
  if(privatePath.test(name)||privateFile.test(name)||/(^|\/)AGENTS\.md$/i.test(name)||
    /(^|\/)(?:cxpalm\.json|local\.properties|desktop-preferences\.json|auth\.json|credentials\.json)$/i.test(name)||
    /(^|\/)\.env(?:\..+)?$/i.test(name)&&!name.endsWith('.env.example')) findings.push({file,rule:'private-or-generated-file'});
  if(bytes.includes(0)&&!/\.(?:png|ico)$/i.test(name)) findings.push({file,rule:'unexpected-binary-manual-review'});
  if(!bytes.includes(0)) {
    const lines=bytes.toString('utf8').split(/\r?\n/);
    const normalizedHome=home.replace(/\\+/g,'/').toLowerCase();
    lines.forEach((text,index)=>{
      for(const [rule,pattern] of patterns) if(pattern.test(text)) findings.push({file,line:index+1,rule});
      const normalized=text.replace(/\\+/g,'/').toLowerCase();
      if(normalizedHome.length>3&&normalized.includes(normalizedHome)) findings.push({file,line:index+1,rule:'local-home-path'});
    });
  }
  return findings;
}

function main() {
  const root=path.resolve(__dirname,'..');
  const names=[...new Set(execFileSync('git',['ls-files','--cached','--others','--exclude-standard','-z'],{cwd:root,maxBuffer:8<<20}).toString('utf8').split('\0').filter(Boolean))].sort();
  const findings=[];
  let count=0;
  for(const file of names) {
    const absolute=path.resolve(root,file);
    const relative=path.relative(root,absolute);
    if(relative.startsWith('..')||path.isAbsolute(relative)) throw Error('Candidate escapes repository');
    if(!fs.existsSync(absolute)) {findings.push({file,rule:'missing-candidate-review-index'});continue;}
    const stat=fs.lstatSync(absolute);
    if(!stat.isFile()||stat.isSymbolicLink()) {findings.push({file,rule:'unsupported-file-type'});continue;}
    if(stat.size>4*1024*1024) {findings.push({file,rule:'oversized-file-manual-review'});continue;}
    findings.push(...inspect(file,fs.readFileSync(absolute)));
    count++;
  }
  console.log(JSON.stringify({scope:'working-tree Git candidates; not staged blobs or history',filesChecked:count,findings},null,2));
  if(findings.length) process.exitCode=1;
}
if(require.main===module) main();
module.exports={inspect};
