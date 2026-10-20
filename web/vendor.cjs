const fs=require('node:fs'),path=require('node:path');
const root=__dirname;
for(const [source,target] of [['marked/lib/marked.esm.js','marked-vendor.mjs'],['dompurify/dist/purify.es.mjs','purify-vendor.mjs']]){
 const input=path.join(root,'node_modules',source);
 fs.writeFileSync(path.join(root,target),fs.readFileSync(input,'utf8').replace(/\/\/# sourceMappingURL=.*$/gm,''));
}
fs.writeFileSync(path.join(root,'MARKDOWN-LICENSES.txt'),['marked','dompurify'].map(name=>name+'\n'+fs.readFileSync(path.join(root,'node_modules',name,'LICENSE'),'utf8')).join('\n\n'));
