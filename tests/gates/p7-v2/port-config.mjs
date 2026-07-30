import assert from 'node:assert/strict';
import { assertLoadHostSafe, resolveP7V2PortConfig } from '../../../scripts/p7-v2-lib.mjs';

const defaults = resolveP7V2PortConfig({});
assert.deepEqual(defaults, {
  host: '127.0.0.1',
  port: 8080,
  appHttpAddr: '127.0.0.1:8080',
  baseUrl: 'http://127.0.0.1:8080',
  env: { P7_V2_API_HOST: '127.0.0.1', P7_V2_API_PORT: '8080', APP_HTTP_ADDR: '127.0.0.1:8080', P7_BASE_URL: 'http://127.0.0.1:8080' },
});
const migrated = resolveP7V2PortConfig({ P7_V2_API_HOST: '127.0.0.1', P7_V2_API_PORT: '18080' });
assert.equal(migrated.baseUrl, 'http://127.0.0.1:18080');
assert.throws(() => resolveP7V2PortConfig({ P7_V2_API_HOST: '172.22.144.1', P7_V2_API_PORT: '18080' }));
assert.throws(() => resolveP7V2PortConfig({ P7_V2_API_PORT: '80' }));
assert.equal(assertLoadHostSafe('http://127.0.0.1:18080').length, 0);
assert.ok(assertLoadHostSafe('http://172.22.144.1:18080').length > 0);
console.log(JSON.stringify({ status: 'passed', fixtures: 6 }));
