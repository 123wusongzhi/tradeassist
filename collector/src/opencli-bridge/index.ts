import { syncTradeMindOpenCliAdapter } from './adapter-install.js';
import { getOpenCliBridgeConfig } from './config.js';
import { listenOpenCliBridge } from './server.js';

const adapter = syncTradeMindOpenCliAdapter();
if (adapter.updated) {
  console.info('[opencli-bridge] TradeMind tmall adapter synchronized.');
}
const server = listenOpenCliBridge(getOpenCliBridgeConfig());

function shutdown(): void {
  console.info('[opencli-bridge] shutting down...');
  server.close(() => process.exit(0));
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);
