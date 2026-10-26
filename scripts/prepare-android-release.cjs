// Stages a manifest locally; APK signing/identity verification and upload are separate.
const fs=require('node:fs'),path=require('node:path'),{createHash}=require('node:crypto');
const {RELEASES}=require('../desktop/updater.cjs'),{atomicJSON}=require('../desktop/settings.cjs');
(async()=>{
 const root=path.resolve(__dirname,'..'),gradle=fs.readFileSync(path.join(root,'android/app/build.gradle.kts'),'utf8');
 const version=/versionName\s*=\s*"([^"]+)"/.exec(gradle)?.[1],build=Number(/versionCode\s*=\s*(\d+)/.exec(gradle)?.[1]);
 const packageName=/applicationId\s*=\s*"([^"]+)"/.exec(gradle)?.[1];
 if(!/^\d+\.\d+\.\d+$/.test(version)||!Number.isSafeInteger(build)||build<1||!packageName)throw Error('Invalid Android metadata');
 const filename=`CXPalm-${version}-build${build}-debug.apk`,source=path.resolve(process.argv[2]||path.join(root,'android/app/build/outputs/apk/debug/app-debug.apk'));
 const stat=fs.statSync(source);if(!stat.isFile()||stat.size<1||stat.size>128*1024*1024)throw Error('APK missing or oversized');
 const hash=createHash('sha256');for await(const chunk of fs.createReadStream(source))hash.update(chunk);const sha256=hash.digest('hex');
 const manifest={version,versionCode:build,package:packageName+'.debug',apk:`${RELEASES}/v${version}-build${build}/${filename}`,size:stat.size,sha256,notes:fs.readFileSync(path.join(root,'deploy/desktop-notes.txt'),'utf8').trim()};
 const directory=path.join(root,'.local',`android-release-${version}-build${build}`);fs.mkdirSync(directory,{recursive:true});const target=path.join(directory,filename);
 if(fs.existsSync(target)){const old=createHash('sha256');for await(const chunk of fs.createReadStream(target))old.update(chunk);if(old.digest('hex')!==sha256)throw Error('Refusing to overwrite immutable APK');}
 else fs.copyFileSync(source,target,fs.constants.COPYFILE_EXCL);
 atomicJSON(path.join(directory,'download.json'),manifest);fs.writeFileSync(path.join(directory,'SHA256SUMS-android.txt'),`${sha256}  ${filename}\n`);
 console.log(JSON.stringify({version,build,size:stat.size,sha256,stagedOnly:true}));
})().catch(error=>{console.error(error.message);process.exitCode=1;});
