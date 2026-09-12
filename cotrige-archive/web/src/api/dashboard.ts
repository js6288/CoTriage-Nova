import { http } from './http';

export interface DashboardSummary {
  raw_total: number;
  inferred_total: number;
  pending: number;
  dlq_total: number;
  ingest_rate_1m: number;
  infer_rate_1m: number;
  p95_latency_ms: number;
  abnormal_rate: number;
  fallback_rate: number;
  classification: Record<string, number>;
  paths: Record<string, number>;
  generated_at: string;
}

export interface TimeseriesPoint {
  time: string;
  inferred: number;
  abnormal: number;
}

export interface InferredEvent {
  event_id: string;
  source?: string;
  equipment: string;
  event_ts: string;
  inferred_at: string;
  raw_log?: string;
  template_id: string;
  template_text: string;
  edge_pred: string;
  severity: number;
  confidence: number;
  uncertainty: number;
  evidence: string[];
  advice_code?: string;
  advice?: string;
  inference_path: string;
  model_version?: string;
  rule_version?: string;
  latency_ms: number;
  stage_latency_ms: Record<string, number>;
}

export interface RouterSummary {
  routed_total: number;
  local_total: number;
  cloud_total: number;
  defer_total: number;
  local_ratio: number;
  cloud_ratio: number;
  defer_ratio: number;
  outbox_pending: number;
  outbox_retry: number;
  outbox_dlq: number;
  p95_route_latency_ms: number;
  average_score: number;
  average_threshold: number;
  last_action: string;
  last_routed_at?: string;
  latest_state_snapshot_at?: string;
  latest_state_snapshot_version?: string;
}

export interface RouterSummaryResponse {
  degraded: boolean;
  summary: RouterSummary;
}

export interface RouterDecision {
  id: number;
  event_id: string;
  trace_id: string;
  source: string;
  equipment: string;
  template_id: string;
  edge_pred: string;
  severity: number;
  confidence: number;
  edge_uncertainty: number;
  inference_path: string;
  action: 'LOCAL' | 'CLOUD' | 'DEFER' | string;
  reason: string;
  score: number;
  threshold: number;
  cloud_health: string;
  raise_version: string;
  latency_ms: number;
  routed_at: string;
}

export interface CloudOutboxItem {
  id: number;
  event_id: string;
  request_id: string;
  priority_score: number;
  defer_reason: string;
  status: string;
  attempt: number;
  max_attempts: number;
  next_retry_at?: string;
  deadline_at?: string;
  created_at: string;
  updated_at: string;
  last_error: string;
}

export interface CloudFeedback {
  id: number;
  event_id: string;
  request_id: string;
  source?: string;
  equipment?: string;
  template_id?: string;
  edge_pred?: string;
  edge_confidence?: number;
  cloud_model: string;
  cloud_pred: string;
  root_cause: string;
  recommendation: string;
  explanation: string;
  cloud_confidence: number;
  cloud_latency_ms: number;
  token_usage: number;
  cost: number;
  reward: number;
  reward_reason: string;
  corrected_edge: boolean;
  added_actionable_info: boolean;
  confirmed_high_risk: boolean;
  processed_by_router?: boolean;
  processed_at?: string;
  created_at: string;
}

export interface RouterStateResponse {
  available: boolean;
  item?: Record<string, unknown>;
  payload?: Record<string, unknown>;
}

export interface CompetitionMetrics {
  schema_version: string;
  window: string;
  collected_at: string;
  realtime: {
    end_to_end: { count: number; avg_ms: number; p50_ms: number; p95_ms: number };
    edge_processing: { count: number; avg_ms: number; p50_ms: number; p95_ms: number };
    by_path: Record<string, { count: number; avg_ms: number; p50_ms: number; p95_ms: number }>;
    cloud_average_ms: number;
    improvement_vs_cloud_pct?: number;
  };
  effect: {
    ground_truth_evaluation: { status: string; reason?: string; sample_count: number };
    cloud_comparison: { status: string; reason?: string; sample_count: number };
    edge_cloud_agreement_rate: number;
    edge_cloud_conflict_rate: number;
    edge_correction_rate: number;
    actionable_gain_rate: number;
    average_reward: number;
  };
  efficiency: {
    routed_total: number; local_total: number; cloud_total: number;
    cloud_offload_rate: number; estimated_upload_reduction_rate: number;
    recent_input_bytes: number; estimated_avoided_upload_bytes: number;
    cloud_token_total: number; cloud_cost_total: number;
  };
  scalability: {
    distinct_sources: number; distinct_equipment: number; observed_inference_paths: number;
    scenario_evidence: { status: string; reason?: string; sample_count: number };
  };
  stability: {
    completion_rate: number; fallback_rate: number; dlq_total: number;
    outbox_success_rate: number; outbox_retry: number; outbox_dlq: number;
    network_experiment: { status: string; reason?: string; sample_count: number };
  };
  consistency: {
    paired_samples: number; conflict_count: number; conflict_rate: number;
    resolved_count: number; resolution_success_rate: number;
    multi_node_experiment: { status: string; reason?: string; sample_count: number };
  };
  evidence: { mechanisms: string[]; application_signals: string[] };
}

export interface AbnormalCluster {
  cluster_id: string;
  edge_pred: string;
  template_id: string;
  template_text: string;
  sample_raw_log: string;
  count: number;
  affected_equipments: number;
  equipments: string[];
  inference_paths: string[];
  primary_inference_path: string;
  max_severity: number;
  avg_confidence: number;
  first_seen: string;
  last_seen: string;
  advice?: string;
  advice_code?: string;
  evidence?: string[];
  priority_score: number;
}

export interface AbnormalAnalysis {
  risk_level: 'LOW' | 'MEDIUM' | 'HIGH';
  abnormal_1m: number;
  abnormal_5m: number;
  growth_ratio: number;
  top_types: Record<string, number>;
  top_clusters: AbnormalCluster[];
  priority_queue: AbnormalCluster[];
  generated_at: string;
}

export async function getDashboardSummary(): Promise<DashboardSummary> {
  const response = await http.get<DashboardSummary>('/api/v1/dashboard/summary');
  return {
    ...response.data,
    classification: response.data.classification ?? {},
    paths: response.data.paths ?? {}
  };
}

export async function getDashboardTimeseries(): Promise<TimeseriesPoint[]> {
  const response = await http.get<{ items: TimeseriesPoint[] }>('/api/v1/dashboard/timeseries');
  return response.data.items ?? [];
}

export async function getAbnormalAnalysis(): Promise<AbnormalAnalysis> {
  const response = await http.get<AbnormalAnalysis>('/api/v1/dashboard/abnormal-analysis');
  return {
    ...response.data,
    top_types: response.data.top_types ?? {},
    top_clusters: response.data.top_clusters ?? [],
    priority_queue: response.data.priority_queue ?? []
  };
}

export async function listEvents(limit = 50): Promise<InferredEvent[]> {
  const response = await http.get<{ items: InferredEvent[] }>('/api/v1/events', { params: { limit } });
  return response.data.items ?? [];
}

export async function getRouterSummary(): Promise<RouterSummaryResponse> {
  const response = await http.get<RouterSummaryResponse>('/api/v1/router/summary');
  return response.data;
}

export async function listRouterDecisions(limit = 50): Promise<RouterDecision[]> {
  const response = await http.get<{ items: RouterDecision[] }>('/api/v1/router/decisions', { params: { limit } });
  return response.data.items ?? [];
}

export async function listRouterOutbox(limit = 20): Promise<CloudOutboxItem[]> {
  const response = await http.get<{ items: CloudOutboxItem[] }>('/api/v1/router/outbox', { params: { limit } });
  return response.data.items ?? [];
}

export async function listCloudFeedback(limit = 80): Promise<CloudFeedback[]> {
  const response = await http.get<{ items: CloudFeedback[] }>('/api/v1/router/cloud-feedback', { params: { limit } });
  return response.data.items ?? [];
}

export async function getRouterState(type = ''): Promise<RouterStateResponse> {
  const response = await http.get<RouterStateResponse>('/api/v1/router/state', { params: type ? { type } : undefined });
  return response.data;
}

export async function getCompetitionMetrics(): Promise<CompetitionMetrics> {
  const response = await http.get<CompetitionMetrics>('/api/v1/competition-metrics');
  return response.data;
}

export function subscribeRouterEvents(onEvent: (event: RouterDecision | Record<string, unknown>) => void): EventSource {
  const source = new EventSource('/api/v1/router/stream');
  source.addEventListener('routed', (message) => {
    onEvent(JSON.parse((message as MessageEvent).data));
  });
  return source;
}

export function subscribeEvents(onEvent: (event: InferredEvent) => void): EventSource {
  const source = new EventSource('/api/v1/events/stream');
  source.addEventListener('inferred', (message) => {
    onEvent(JSON.parse((message as MessageEvent).data) as InferredEvent);
  });
  return source;
}
