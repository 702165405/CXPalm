// Prepares local release files only. Publishing is a separate authorized operation.
const fs=require('node:fs'),path=require('node:path'),{createHash}=require('node:crypto');
const {validateManifest,RELEASES}=require('../desktop/updater.cjs'),{atomicJSON}=require('../desktop/settings.cjs');
(async()=>{
 const root=path.resolve(__dirname,'..'),metadata=require('../desktop/package.json'),version=metadata.version,build=metadata.releaseBuild;
 if(!Number.isSafeInteger(build)||build<1)throw new Error('Release build is required');
 const filename=`CXPalm-Setup-${version}-build${build}-x64.exe`,source=path.resolve(process.argv[2]||path.join(root,'dist','desktop-'+version+'-build'+build,filename));
 const stat=fs.statSync(source);if(!stat.isFile())throw new Error('Installer missing');
 const hash=createHash('sha256');for await(const chunk of fs.createReadStream(source))hash.update(chunk);const sha256=hash.digest('hex');
 const manifest={schema:1,releaseLine:'fresh-001',build,appId:'dev.cxpalm.desktop',platform:'win32',arch:'x64',version,url:RELEASES+`/v${version}-build${build}/`+filename,size:stat.size,sha256,notes:fs.readFileSync(path.join(root,'deploy/desktop-notes.txt'),'utf8').trim()};validateManifest(manifest);
 const directory=path.join(root,'.local','desktop-release-'+version+'-build'+build);fs.mkdirSync(directory,{recursive:true});const target=path.join(directory,filename);
 if(fs.existsSync(target)){const old=createHash('sha256');for await(const chunk of fs.createReadStream(target))old.update(chunk);if(old.digest('hex')!==sha256)throw new Error('Refusing to overwrite a different immutable installer for this version');}
 else fs.copyFileSync(source,target,fs.constants.COPYFILE_EXCL);
 atomicJSON(path.join(directory,'desktop-download.json'),manifest);fs.writeFileSync(path.join(directory,'SHA256SUMS-desktop.txt'),sha256+'  '+filename+'\n');console.log('Local release staged, NOT published: '+directory);console.log(JSON.stringify({version,size:stat.size,sha256}));
})().catch(error=>{console.error(error.message);process.exitCode=1;});
