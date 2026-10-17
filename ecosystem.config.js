const os = require('os');
const path = require('path');
const fs = require('fs');

const isWin = os.platform() === 'win32';
const exeName = isWin ? 'antidpi.exe' : 'antidpi';

let scriptPath = path.join(__dirname, 'bin', exeName);
if (!fs.existsSync(scriptPath)) {
  const localPath = path.join(__dirname, exeName);
  if (fs.existsSync(localPath)) {
    scriptPath = localPath;
  }
}

module.exports = {
  apps: [{
    name: 'antidpi',
    script: scriptPath,
    args: '-port=8080 -pac=true -autoproxy=false -dns=false',
    cwd: __dirname,
    autorestart: true,
    watch: false,
    kill_timeout: 4000,
    restart_delay: 2000,
    max_memory_restart: '150M',
    env: {
      NODE_ENV: 'production'
    }
  }]
};
