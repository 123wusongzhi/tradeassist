import { authHeaders, baseUrl, extractCursor } from './guards.js';
import http from 'k6/http';

const paths = {
  productList: '/api/v1/products?pageSize=20',
  orderList: '/api/v1/orders?pageSize=20',
  inventoryList: '/api/v1/inventory?pageSize=20',
  taskList: '/api/v1/task-center/failures?pageSize=20',
  webhookEventList: '/api/v1/webhooks/events?pageSize=20',
  operationLogList: '/api/v1/operation-logs?pageSize=20',
};

export function readList(name, cursor = '') {
  const path = paths[name] || paths.productList;
  const query = cursor ? `${path}&cursor=${encodeURIComponent(cursor)}` : path;
  return http.get(`${baseUrl()}${query}`, {
    headers: authHeaders(),
    tags: { scenario: name, group: 'read-heavy' },
  });
}

export function followCursor(name, maxPages = 2) {
  let cursor = '';
  let last = null;
  for (let i = 0; i < maxPages; i += 1) {
    last = readList(name, cursor);
    cursor = extractCursor(last.body);
    if (!cursor) break;
  }
  return last;
}
