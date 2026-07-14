import http from 'k6/http';
import crypto from 'k6/crypto';
import { baseUrl } from './guards.js';
import { webhookTestSecret } from './credentials.js';

export function signWebhookBody(body, ts = Math.floor(Date.now() / 1000)) {
  const payload = typeof body === 'string' ? body : JSON.stringify(body);
  const secret = webhookTestSecret();
  return {
    body: payload,
    ts,
    sig: crypto.hmac('sha256', secret, `${ts}.${payload}`, 'hex'),
  };
}

export function postSignedWebhook(path, body, tags = {}) {
  const signed = signWebhookBody(body);
  return http.post(`${baseUrl()}${path}`, signed.body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Webhook-Signature': signed.sig,
      'X-Webhook-Timestamp': String(signed.ts),
    },
    tags,
  });
}
