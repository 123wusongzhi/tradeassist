import assert from 'node:assert/strict';
import { compareProcessIdentity, processIdentityKey, verifyInstanceNonce, verifyPortOwner, verifyServerBinary } from '../../../scripts/p7-v2-process-identity.mjs';

const base = { present: true, bootId: 'boot', pid: '100', processStartTicks: '10', executableSha256: 'hash', listeningPort: 8080, portOwnerPid: '100', instanceNonce: 'nonce' };
assert.equal(processIdentityKey(base), 'boot:100:10:hash');
assert.equal(compareProcessIdentity(null, base).freshProcessVerified, true);
assert.equal(compareProcessIdentity(base, { ...base, pid: '101', portOwnerPid: '101' }).processChanged, true);
const reused = compareProcessIdentity(base, { ...base, processStartTicks: '11' });
assert.equal(reused.processChanged, true);
assert.equal(reused.pidReused, true);
assert.equal(compareProcessIdentity(base, base).sameProcess, true);
assert.equal(verifyPortOwner(base, 8080), true);
assert.equal(verifyServerBinary(base, 'hash'), true);
assert.equal(verifyInstanceNonce(base, 'nonce'), true);
assert.equal(verifyInstanceNonce(base, 'other'), false);
console.log(JSON.stringify({ status: 'passed', fixtures: 8 }));
