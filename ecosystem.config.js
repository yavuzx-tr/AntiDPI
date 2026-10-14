const os = require('os');
const path = require('path');

const isWin = os.platform() === 'win32';

module.exports = {
  apps: [{
    name: 'antidpi',
    script: isWin ? path.join(__dirname, 'bin', 'antidpi.exe') : path.join(__dirname, 'run.sh'),
    args: isWin ? '-pac=true -autoproxy=false -dns=false' : '',
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
