import { http } from './http';

export type EdgePred = 'NORMAL' | 'ABNORMAL' | 'APP_ERROR' | 'NET_ERROR' | 'DISK_ERROR' | 'INFRA_ERROR' | 'SECURITY' | 'UNKNOWN';
export type KnowledgeStatus = 'draft' | 'published' | 'disabled' | 'deleted';

export interface EdgeRule {
  id: number;
  name: string;
  description: string;
  match_type: string;
  pattern: string;
  keywords: string[];
  target_field: string;
  edge_pred: EdgePred;
  severity: number;
  confidence: number;
  evidence: string[];
  advice_code: string;
  advice: string;
  priority: number;
  enabled: boolean;
  status: KnowledgeStatus;
  updated_at: string;
}

export interface EdgeTemplate {
  id: number;
  template_id: string;
  template_text: string;
  raw_example: string;
  source: string;
  edge_pred: EdgePred;
  severity: number;
  confidence: number;
  evidence: string[];
  advice_code: string;
  advice: string;
  status: KnowledgeStatus;
  updated_at: string;
}

export interface ListResponse<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export type RulePayload = Omit<EdgeRule, 'id' | 'updated_at'>;
export type TemplatePayload = Omit<EdgeTemplate, 'id' | 'template_id' | 'updated_at'>;

export interface TemplateFromClusterPayload {
  template_text: string;
  sample_raw_log?: string;
  source?: string;
  edge_pred: EdgePred;
  max_severity: number;
  avg_confidence: number;
  evidence?: string[];
  advice_code?: string;
  advice?: string;
  status?: KnowledgeStatus;
}

export async function listRules(params = {}) {
  const res = await http.get<ListResponse<EdgeRule>>('/api/v1/edge-rules', { params });
  return res.data;
}

export async function createRule(data: RulePayload) {
  const res = await http.post<EdgeRule>('/api/v1/edge-rules', data);
  return res.data;
}

export async function updateRule(id: number, data: RulePayload) {
  const res = await http.patch<EdgeRule>(`/api/v1/edge-rules/${id}`, data);
  return res.data;
}

export async function deleteRule(id: number) {
  await http.delete(`/api/v1/edge-rules/${id}`);
}

export async function activateRuleSet(name: string, item_ids: number[]) {
  const set = await http.post<{ id: number }>('/api/v1/edge-rule-sets', { name, item_ids });
  const res = await http.post(`/api/v1/edge-rule-sets/${set.data.id}/activate`);
  return res.data;
}

export async function listTemplates(params = {}) {
  const res = await http.get<ListResponse<EdgeTemplate>>('/api/v1/edge-templates', { params });
  return res.data;
}

export async function createTemplate(data: TemplatePayload) {
  const res = await http.post<EdgeTemplate>('/api/v1/edge-templates', data);
  return res.data;
}

export async function createTemplateFromCluster(data: TemplateFromClusterPayload) {
  const res = await http.post<{ created: boolean; item: EdgeTemplate }>('/api/v1/edge-templates/from-cluster', data);
  return res.data;
}

export async function updateTemplate(id: number, data: TemplatePayload) {
  const res = await http.patch<EdgeTemplate>(`/api/v1/edge-templates/${id}`, data);
  return res.data;
}

export async function deleteTemplate(id: number) {
  await http.delete(`/api/v1/edge-templates/${id}`);
}

export async function activateTemplateSet(name: string, item_ids: number[]) {
  const set = await http.post<{ id: number }>('/api/v1/edge-template-sets', { name, item_ids });
  const res = await http.post(`/api/v1/edge-template-sets/${set.data.id}/activate`);
  return res.data;
}
