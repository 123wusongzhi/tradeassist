import assert from 'node:assert/strict';
import { compareProcessIdentity, verifyPortOwner } from '../../../scripts/p7-v2-process-identity.mjs';

const owner = { present: true, pid: '42', portOwnerPid: '42', listeningPort: 18080, bootId: 'boot', processStartTicks: '1', executableSha256: 'hash' };
assert.equal(verifyPortOwner(owner, 18080), true);
assert.equal(verifyPortOwner({ ...owner, portOwnerPid: '43' }, 18080), false);
assert.equal(verifyPortOwner({ ...owner, listeningPort: 8080 }, 18080), false);
assert.equal(compareProcessIdentity(owner, { ...owner, processStartTicks: '2' }).pidReused, true);
assert.equal(compareProcessIdentity(owner, { ...owner, pid: '43', portOwnerPid: '43' }).freshProcessVerified, true);
assert.equal(compareProcessIdentity(owner, null).freshProcessVerified, false);
console.log(JSON.stringify({ status: 'passed', fixtures: 6, categories: ['free_port', 'stale_pid', 'owner_mismatch', 'pid_reuse'] }));
