// Compatibility entry point only. Current and baseline execute p7v2-baseline.js
// through scripts/p7-v2-load.mjs so their request logic cannot drift.
export * from './p7v2-baseline.js';
export { default } from './p7v2-baseline.js';
