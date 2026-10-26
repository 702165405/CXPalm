const minimums = new Map([[22, [23, 4]], [24, [22, 1]], [26, [12, 1]]]);

function checkNodeVersion(version = process.versions.node) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(version);
  const [major, minor, patch] = match ? match.slice(1).map(Number) : [];
  const minimum = minimums.get(major);
  if (!minimum || minor < minimum[0] || minor === minimum[0] && patch < minimum[1]) {
    throw new Error(`Unsupported build Node.js ${version}. Use Node 22.23.4+, 24.22.1+ or 26.12.1+ on those release lines. Updating system Node does not update Electron's embedded runtime.`);
  }
}

if (require.main === module) {
  try {
    checkNodeVersion();
    console.log(`Build Node.js ${process.versions.node}: security baseline satisfied (not an Electron runtime check).`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = {checkNodeVersion};
