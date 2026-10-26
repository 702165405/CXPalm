const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
test('published clients, gateway and manifests share one version and build',()=>{
 const root=path.resolve(__dirname,'..'),read=file=>fs.readFileSync(path.join(root,file),'utf8'),json=file=>JSON.parse(read(file));
 const desktop=json('desktop/package.json'),lock=json('desktop/package-lock.json'),android=read('android/app/build.gradle.kts');
 assert.match(desktop.version,/^\d+\.\d+\.\d+$/);
 assert.equal(lock.version,desktop.version);assert.equal(lock.packages[''].version,desktop.version);
 assert.equal(/versionName\s*=\s*"([^"]+)"/.exec(android)[1],desktop.version);
 assert.equal(Number(/versionCode\s*=\s*(\d+)/.exec(android)[1]),desktop.releaseBuild);
 assert.ok(Number.isSafeInteger(desktop.releaseBuild)&&desktop.releaseBuild>=1);
 assert.equal(/const Version = "([^"]+)"/.exec(read('internal/config/config.go'))[1],desktop.version);
 const apk=json('updates/download.json'),win=json('updates/desktop-download.json');
 assert.equal(apk.version,desktop.version);assert.equal(win.version,desktop.version);
 assert.equal(apk.versionCode,desktop.releaseBuild);assert.equal(win.build,desktop.releaseBuild);
 assert.equal(win.releaseLine,'fresh-001');
});
