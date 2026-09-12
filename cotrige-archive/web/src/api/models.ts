import { http } from './http';

export type ModelStatus = 'draft' | 'published' | 'disabled' | 'deleted';
export type ProviderType = 'llama_cpp' | 'openai_compat' | 'vllm' | 'custom_http';
export type CloudProviderType = 'openai_compat' | 'deepseek' | 'qwen' | 'custom_http';

export interface ClassifierConfig {
  id: number;
  name: string;
  endpoint: string;
  model: string;
  timeout_ms: number;
  threshold: number;
  status: ModelStatus;
  is_active: boolean;
  config_version: number;
  etag: string;
  updated_at: string;
}

export interface ClassifierPayload {
  name: string;
  endpoint: string;
  model: string;
  timeout_ms: number;
  threshold: number;
  status: ModelStatus;
}

export interface EdgeProvider {
  id: number;
  name: string;
  provider: ProviderType;
  endpoint: string;
  model: string;
  timeout_ms: number;
  max_tokens: number;
  temperature: number;
  top_p: number;
  seed?: number | null;
  healthcheck_path?: string | null;
  response_path: string;
  status: ModelStatus;
  is_active: boolean;
  config_version: number;
  etag: string;
  updated_at: string;
}

export interface ProviderPayload {
  name: string;
  provider: ProviderType;
  endpoint: string;
  model: string;
  timeout_ms: number;
  max_tokens: number;
  temperature: number;
  top_p: number;
  seed?: number | null;
  healthcheck_path?: string | null;
  response_path: string;
  status?: ModelStatus;
}

export interface CloudProvider {
  id: number;
  name: string;
  provider: CloudProviderType;
  endpoint: string;
  model: string;
  api_key_env: string;
  api_key: string;
  timeout_ms: number;
  max_tokens: number;
  temperature: number;
  top_p: number;
  seed?: number | null;
  health_path?: string | null;
  response_path: string;
  status: ModelStatus;
  is_active: boolean;
  config_version: number;
  etag: string;
  updated_at: string;
  last_publish_at?: string | null;
}

export interface CloudProviderPayload {
  name: string;
  provider: CloudProviderType;
  endpoint: string;
  model: string;
  api_key_env: string;
  api_key?: string;
  timeout_ms: number;
  max_tokens: number;
  temperature: number;
  top_p: number;
  seed?: number | null;
  health_path?: string | null;
  response_path: string;
  status?: ModelStatus;
}

export interface CloudConfig {
  enabled: boolean;
  reason?: string;
  config_version?: number;
  etag?: string;
  provider?: CloudProvider;
  generated_at: string;
}

interface ListResponse<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export async function listClassifiers(params = {}) {
  const res = await http.get<ListResponse<ClassifierConfig>>('/api/v1/edge-classifiers', { params });
  return res.data;
}

export async function createClassifier(data: ClassifierPayload) {
  const res = await http.post<ClassifierConfig>('/api/v1/edge-classifiers', data);
  return res.data;
}

export async function updateClassifier(id: number, data: ClassifierPayload) {
  const res = await http.patch<ClassifierConfig>(`/api/v1/edge-classifiers/${id}`, data);
  return res.data;
}

export async function deleteClassifier(id: number) {
  await http.delete(`/api/v1/edge-classifiers/${id}`);
}

export async function activateClassifier(id: number) {
  const res = await http.post<ClassifierConfig>(`/api/v1/edge-classifiers/${id}/activate`);
  return res.data;
}

export async function listProviders(params = {}) {
  const res = await http.get<ListResponse<EdgeProvider>>('/api/v1/edge-providers', { params });
  return res.data;
}

export async function createProvider(data: ProviderPayload) {
  const res = await http.post<EdgeProvider>('/api/v1/edge-providers', data);
  return res.data;
}

export async function updateProvider(id: number, data: Partial<ProviderPayload>) {
  const res = await http.patch<EdgeProvider>(`/api/v1/edge-providers/${id}`, data);
  return res.data;
}

export async function deleteProvider(id: number) {
  await http.delete(`/api/v1/edge-providers/${id}`);
}

export async function activateProvider(id: number) {
  const res = await http.post(`/api/v1/edge-providers/${id}/activate`);
  return res.data;
}

export async function listCloudProviders() {
  const res = await http.get<ListResponse<CloudProvider>>('/api/v1/cloud-providers');
  return res.data;
}

export async function createCloudProvider(data: CloudProviderPayload) {
  const res = await http.post<CloudProvider>('/api/v1/cloud-providers', data);
  return res.data;
}

export async function updateCloudProvider(id: number, data: Partial<CloudProviderPayload>) {
  const res = await http.patch<CloudProvider>(`/api/v1/cloud-providers/${id}`, data);
  return res.data;
}

export async function deleteCloudProvider(id: number) {
  await http.delete(`/api/v1/cloud-providers/${id}`);
}

export async function activateCloudProvider(id: number) {
  const res = await http.post<CloudProvider>(`/api/v1/cloud-providers/${id}/activate`);
  return res.data;
}

export async function getCloudConfig() {
  const res = await http.get<CloudConfig>('/api/v1/cloud-config');
  return res.data;
}
