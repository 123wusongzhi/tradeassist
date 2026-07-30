import { syncTradeMindOpenCliAdapter } from '../opencli-bridge/adapter-install.js';

const result = syncTradeMindOpenCliAdapter();
console.info(
  result.updated
    ? '[opencli-adapter] TradeMind tmall adapter installed.'
    : '[opencli-adapter] TradeMind tmall adapter is already up to date.',
);
