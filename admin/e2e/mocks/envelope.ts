export type ApiEnvelope<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export function ok<T>(data: T): ApiEnvelope<T> {
  return { code: 0, message: 'ok', data, traceId: 'e2e-trace' };
}

export function fail<T = null>(message: string, code = 400, data: T | null = null): ApiEnvelope<T | null> {
  return { code, message, data, traceId: 'e2e-trace' };
}

export function paged<T>(items: T[], total = items.length, page = 1, pageSize = 20) {
  return {
    list: items,
    pagination: {
      page,
      pageSize,
      total,
      totalPages: Math.max(1, Math.ceil(total / pageSize)),
    },
  };
}
