export type User = {id: string; role: 'SUPER_ADMIN_ROOT' | 'OWNER_LIBRARY'; libraryId: string | null; passwordChangeRequired: boolean};
export type Candidate = {bookId: number; rank: number; title: string; ftsScore: number};
export type Job = {id: string; status: string; nsfwDecision?: string; decisionCode?: string; failureCode?: string; targetBookId?: number; contentType: string};
export type Batch = {id: string; libraryId: string; status: string; totalFiles: number; createdAt: string; jobs: Job[]};
export type Review = {id: string; libraryId: string; status: string; nsfwDecision: string; candidates: Candidate[]};
export type Owner = {username: string; library: {id: string; name: string; status: string}};

type HTTP = {
  enableSessionRefresh(): void;
  clearSession(): void;
  profile(): Promise<User>;
  json(path: string, options?: RequestInit): Promise<unknown>;
  request(path: string, options?: RequestInit): Promise<Response>;
  authJSON(action: 'logout'): Promise<unknown>;
};

declare global { interface Window { DeftaHTTP: HTTP } }

export const http = window.DeftaHTTP;
export function scope(libraryId: string): string {return libraryId ? `?libraryId=${encodeURIComponent(libraryId)}` : '';}
export async function batches(libraryId: string): Promise<Batch[]> {
  const payload = await http.json(`/api/manage/cover-imports${scope(libraryId)}${libraryId ? '&' : '?'}limit=30`) as {results: Batch[]};
  return payload.results;
}
export async function libraries(): Promise<Owner[]> {
  const result: Owner[] = [];
  for (let offset = 0; ; offset += 100) {
    const payload = await http.json(`/api/admin/owners?offset=${offset}&limit=100`) as {results: Owner[]; total: number};
    result.push(...payload.results);
    if (result.length >= payload.total || payload.results.length === 0) return result.filter(owner => owner.library?.status === 'ACTIVE');
  }
}
export async function upload(files: File[], libraryId: string, key: string): Promise<void> {
  const body = new FormData();
  if (libraryId) body.set('libraryId', libraryId);
  files.forEach(file => body.append('covers', file, file.name));
  await http.json('/api/manage/cover-imports', {method: 'POST', headers: {'Idempotency-Key': key}, body});
}
export function review(id: string, libraryId: string): Promise<Review> {
  return http.json(`/api/manage/cover-imports/${encodeURIComponent(id)}${scope(libraryId)}`) as Promise<Review>;
}
export async function preview(id: string, libraryId: string): Promise<string> {
  const response = await http.request(`/api/manage/cover-imports/${encodeURIComponent(id)}/source${scope(libraryId)}`);
  return URL.createObjectURL(await response.blob());
}
export async function decide(id: string, libraryId: string, action: 'ACCEPT' | 'REJECT', bookId?: number): Promise<void> {
  await http.json(`/api/manage/cover-imports/${encodeURIComponent(id)}/decision`, {
    method: 'POST', body: JSON.stringify({libraryId, action, ...(action === 'ACCEPT' ? {bookId} : {})})
  });
}
export async function quarantine(id: string, libraryId: string, decision: 'APPROVE' | 'REJECT'): Promise<void> {
  await http.json(`/api/manage/cover-imports/${encodeURIComponent(id)}/quarantine-decision`, {
    method: 'POST', body: JSON.stringify({libraryId, decision})
  });
}
