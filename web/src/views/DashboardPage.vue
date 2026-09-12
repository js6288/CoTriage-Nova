<template>
  <div class="dashboard">
    <div class="page-head">
      <div>
        <span class="product-eyebrow">CoTriage · Edge–Cloud Intelligence</span>
        <h1>云边协同智能运维平台</h1>
        <p>边缘实时感知、动态决策与云端根因诊断</p>
      </div>
      <div class="connection">
        <div class="stream-state">
          <span :class="['pulse', streamConnected ? 'online' : 'offline']"></span>
          {{ streamConnected ? '实时流已连接' : '实时流连接中' }}
        </div>
        <a-tooltip title="返回首页">
          <a-button class="nav-icon-btn" type="text" @click="goHome" aria-label="返回首页">
            <HomeOutlined />
          </a-button>
        </a-tooltip>
        <a-button type="primary" @click="goManage">进入管理页面</a-button>
      </div>
    </div>

    <div class="section-head">
      <div><span>01</span><div><b>运行概览</b><small>关键运行状态与服务效能</small></div></div>
      <span class="section-status">数据窗口 {{ competitionMetrics.window || '采集中' }}</span>
    </div>

    <div class="metrics">
      <MetricCard label="近一分钟接入" :value="summary.ingest_rate_1m" suffix="条" />
      <MetricCard label="边缘处理 P95" :value="competitionMetrics.realtime.edge_processing.p95_ms" suffix="ms" />
      <MetricCard label="推理完成率" :value="percent(competitionMetrics.stability.completion_rate)" suffix="%" />
      <MetricCard label="异常日志比例" :value="percent(summary.abnormal_rate)" suffix="%" />
      <MetricCard label="降级比例" :value="percent(summary.fallback_rate)" suffix="%" :warning="summary.fallback_rate > 0.2" />
      <MetricCard label="边缘闭环比例" :value="percent(competitionMetrics.efficiency.cloud_offload_rate)" suffix="%" />
      <MetricCard label="云端复核比例" :value="percent(routerSummary.cloud_ratio)" suffix="%" />
      <MetricCard label="Defer 队列" :value="routerSummary.outbox_pending + routerSummary.outbox_retry" suffix="条" :warning="routerSummary.outbox_pending + routerSummary.outbox_retry > 0" />
    </div>

    <a-card :bordered="false" class="panel competition-panel">
      <template #title>
        <div class="panel-title">
          <div><small class="panel-kicker">EDGE–CLOUD PERFORMANCE</small><span>云边协同效能</span></div>
          <span class="event-count">更新于 {{ timeLabel(competitionMetrics.collected_at) }}</span>
        </div>
      </template>
      <div class="competition-grid">
        <div class="competition-group">
          <h3>实时性</h3>
          <div class="competition-kv"><span>边缘处理 P95</span><b>{{ competitionMetrics.realtime.edge_processing.p95_ms }} ms</b></div>
          <div class="competition-kv"><span>端到端 P95</span><b>{{ competitionMetrics.realtime.end_to_end.p95_ms }} ms</b></div>
          <div class="competition-kv"><span>相对云端改善</span><b>{{ optionalPercent(competitionMetrics.realtime.improvement_vs_cloud_pct) }}</b></div>
        </div>
        <div class="competition-group">
          <h3>效果与一致性</h3>
          <div class="competition-kv"><span>云边配对样本</span><b>{{ competitionMetrics.consistency.paired_samples }}</b></div>
          <div class="competition-kv"><span>云边一致率</span><b>{{ percent(competitionMetrics.effect.edge_cloud_agreement_rate) }}%</b></div>
          <div class="competition-kv"><span>冲突解决率</span><b>{{ percent(competitionMetrics.consistency.resolution_success_rate) }}%</b></div>
        </div>
        <div class="competition-group">
          <h3>资源与通信</h3>
          <div class="competition-kv"><span>云端卸载率</span><b>{{ percent(competitionMetrics.efficiency.cloud_offload_rate) }}%</b></div>
          <div class="competition-kv"><span>估算减少上传</span><b>{{ byteText(competitionMetrics.efficiency.estimated_avoided_upload_bytes) }}</b></div>
          <div class="competition-kv"><span>累计云端 Token</span><b>{{ competitionMetrics.efficiency.cloud_token_total }}</b></div>
        </div>
        <div class="competition-group">
          <h3>稳定性与扩展性</h3>
          <div class="competition-kv"><span>完成率</span><b>{{ percent(competitionMetrics.stability.completion_rate) }}%</b></div>
          <div class="competition-kv"><span>Outbox 成功率</span><b>{{ percent(competitionMetrics.stability.outbox_success_rate) }}%</b></div>
          <div class="competition-kv"><span>设备 / 来源</span><b>{{ competitionMetrics.scalability.distinct_equipment }} / {{ competitionMetrics.scalability.distinct_sources }}</b></div>
        </div>
      </div>
    </a-card>

    <div class="section-head content-section-head"><div><span>02</span><div><b>实时感知与推理</b><small>接入趋势、推理路径与日志分类</small></div></div></div>

    <a-card :bordered="false" class="panel chart-panel trend-panel">
      <template #title>最近 10 分钟处理趋势</template>
      <EChartPanel :option="trendOption" class="trend-chart" />
    </a-card>

    <div class="chart-grid">
      <a-card :bordered="false" class="panel chart-panel">
        <template #title>推理路径分布</template>
        <EChartPanel :option="pathOption" />
      </a-card>

      <a-card :bordered="false" class="panel chart-panel">
        <template #title>日志分类分布</template>
        <EChartPanel :option="classificationOption" />
      </a-card>
    </div>

    <div class="section-head content-section-head"><div><span>03</span><div><b>动态决策与弱网补偿</b><small>RAISE 路由策略与可靠传输状态</small></div></div></div>
    <div class="router-grid">
      <a-card :bordered="false" class="panel router-panel">
        <template #title>
          <div class="panel-title">
            <span>RAISE 路由状态</span>
            <a-tag :color="routerDegraded ? 'orange' : 'green'">{{ routerDegraded ? 'Degraded' : 'Online' }}</a-tag>
          </div>
        </template>
        <div class="router-summary">
          <div class="router-gauge">
            <EChartPanel :option="routerActionOption" class="router-chart" />
          </div>
          <div class="router-explain">
            <a-tooltip title="越接近 1，表示 RAISE 越倾向把事件交给云端复核；低于路由门槛通常会留在本地处理。">
              <div class="router-kv"><span>上云压力</span><b>{{ scoreText(routerSummary.average_score) }}</b></div>
            </a-tooltip>
            <a-tooltip title="动态门槛会随队列积压、预算余量、网络健康和审计比例变化；分数超过门槛才会尝试 CLOUD。">
              <div class="router-kv"><span>路由门槛</span><b>{{ scoreText(routerSummary.average_threshold) }}</b></div>
            </a-tooltip>
            <a-tooltip title="最近一条事件的路由结果：LOCAL 表示本地处理，CLOUD 表示进入云端请求，DEFER 表示先进入弱网补偿队列。">
              <div class="router-kv"><span>最近去向</span><a-tag :color="routeActionColor(routerSummary.last_action)">{{ routerSummary.last_action || '-' }}</a-tag></div>
            </a-tooltip>
            <a-tooltip title="router-controller 最近写入的状态快照版本，用于判断当前页面看到的是哪一版 RAISE 运行态。">
              <div class="router-kv"><span>运行版本</span><b>{{ routerSummary.latest_state_snapshot_version || '-' }}</b></div>
            </a-tooltip>
          </div>
        </div>
        <div class="router-state-preview">
          <div class="state-chip">
            <span>云端健康</span>
            <a-tag :color="cloudHealthColor(systemState.CloudHealth)">{{ systemState.CloudHealth || '-' }}</a-tag>
          </div>
          <div class="state-chip">
            <span>失败率</span>
            <b>{{ percent(systemState.FailRate || 0) }}%</b>
          </div>
          <div class="state-chip">
            <span>队列压力</span>
            <b>{{ systemState.CloudQueueLag || 0 }}/{{ systemState.QueueMax || '-' }}</b>
          </div>
          <div class="state-chip">
            <span>Outbox</span>
            <b>{{ systemState.OutboxLag || 0 }}</b>
          </div>
          <div class="state-chip">
            <span>分钟预算</span>
            <b>{{ numberText(systemState.MinuteBudgetRemaining) }}</b>
          </div>
          <div class="state-chip">
            <span>日预算</span>
            <b>{{ numberText(systemState.DayBudgetRemaining) }}</b>
          </div>
        </div>
      </a-card>

      <a-card :bordered="false" class="panel router-panel">
        <template #title>
          <div class="panel-title">
            <span>Outbox 补偿队列</span>
            <span class="event-count">最近 {{ outboxItems.length }} 条</span>
          </div>
        </template>
        <div v-if="outboxItems.length" class="outbox-list">
          <div v-for="item in outboxItems" :key="item.id" class="outbox-item">
            <div class="outbox-main">
              <a-tag :color="outboxStatusColor(item.status)">{{ item.status }}</a-tag>
              <b>{{ item.event_id }}</b>
              <span>score {{ scoreText(item.priority_score) }}</span>
            </div>
            <div class="outbox-meta">
              <span>attempt {{ item.attempt }}/{{ item.max_attempts || '-' }}</span>
              <span>{{ item.defer_reason || item.last_error || '等待补传' }}</span>
            </div>
          </div>
        </div>
        <a-empty v-else :image="simpleImage" description="暂无 outbox 积压" />
      </a-card>
    </div>

    <div class="section-head content-section-head"><div><span>04</span><div><b>异常诊断中心</b><small>风险聚合、云端根因与实时事件</small></div></div></div>
    <a-card :bordered="false" class="panel abnormal-panel">
      <template #title>
        <div class="panel-title">
          <span>异常态势分析</span>
          <span class="event-count">风险等级 {{ riskText(analysis.risk_level) }}</span>
        </div>
      </template>
      <div class="analysis-summary">
        <div :class="['risk-card', analysis.risk_level.toLowerCase()]">
          <span>当前风险</span>
          <b>{{ riskText(analysis.risk_level) }}</b>
          <small>{{ riskReason }}</small>
        </div>
        <div class="analysis-stat"><span>1分钟异常</span><b>{{ analysis.abnormal_1m }}</b></div>
        <div class="analysis-stat"><span>5分钟异常</span><b>{{ analysis.abnormal_5m }}</b></div>
        <div class="analysis-stat"><span>增长倍数</span><b>{{ ratioText(analysis.growth_ratio) }}</b></div>
      </div>
      <div class="analysis-grid">
        <div>
          <div class="section-title">异常聚合 Top</div>
          <div v-if="analysis.top_clusters.length" class="cluster-list">
            <div v-for="item in analysis.top_clusters" :key="item.cluster_id" class="cluster-card">
              <div class="cluster-head">
                <div>
                  <a-tag :color="predictionColor(item.edge_pred)">{{ item.edge_pred }}</a-tag>
                  <span class="cluster-count">{{ item.count }} 次</span>
                </div>
                <span :class="severityClass(item.max_severity)">Severity {{ item.max_severity }}</span>
              </div>
              <div class="cluster-title">{{ item.template_text }}</div>
              <div class="cluster-log">{{ item.sample_raw_log || item.template_text }}</div>
              <div class="cluster-meta">
                <span>影响设备：{{ equipmentText(item) }}</span>
                <span>推理路径：<a-tag color="blue">{{ item.primary_inference_path || '-' }}</a-tag></span>
                <span>最近 {{ relativeTime(item.last_seen) }}</span>
                <span>置信度 {{ percent(item.avg_confidence) }}%</span>
              </div>
              <div class="cluster-footer">
                <div class="cluster-advice">{{ item.advice || evidenceText(item) || '等待更多诊断信息' }}</div>
                <a-button size="small" type="link" :loading="depositingCluster === item.cluster_id" @click="depositTemplate(item)">沉淀为模板</a-button>
              </div>
            </div>
          </div>
          <a-empty v-else :image="simpleImage" description="暂无异常聚合" />
        </div>
        <div>
          <div class="section-title">优先处理队列</div>
          <div v-if="analysis.priority_queue.length" class="priority-list">
            <div v-for="(item, index) in analysis.priority_queue" :key="item.cluster_id" class="priority-item">
              <div class="priority-rank">P{{ index + 1 }}</div>
              <div class="priority-body">
                <div class="priority-main">
                  <span>{{ item.edge_pred }}</span>
                  <b>{{ item.count }} 次</b>
                </div>
                <div class="priority-desc">{{ item.template_text }}</div>
                <div class="cluster-meta">
                  <span>设备 {{ equipmentText(item) }}</span>
                  <span>路径 {{ item.primary_inference_path || '-' }}</span>
                  <span>最高严重度 {{ item.max_severity }}</span>
                  <span>分数 {{ item.priority_score }}</span>
                </div>
                <div class="priority-actions">
                  <a-button size="small" type="link" :loading="depositingCluster === item.cluster_id" @click="depositTemplate(item)">沉淀为模板</a-button>
                </div>
              </div>
            </div>
          </div>
          <a-empty v-else :image="simpleImage" description="暂无优先处理项" />
        </div>
      </div>
    </a-card>

    <a-card :bordered="false" class="panel cloud-feedback-panel">
      <template #title>
        <div class="panel-title">
          <span>云端根因诊断</span>
          <span class="event-count">最近 {{ cloudFeedbackItems.length }} 条</span>
        </div>
      </template>
      <div v-if="visibleCloudFeedback.length" class="cloud-feedback-list">
        <div v-for="item in visibleCloudFeedback" :key="item.id" class="cloud-feedback-item">
          <div class="cloud-feedback-head">
            <div>
              <a-tag color="geekblue">{{ item.cloud_pred || 'UNKNOWN' }}</a-tag>
              <b>{{ item.equipment || item.source || '未知设备' }}</b>
              <span>{{ item.cloud_model }}</span>
            </div>
            <span>{{ timeLabel(item.created_at) }} · {{ item.cloud_latency_ms }} ms</span>
          </div>
          <div class="cloud-feedback-content">
            <p v-if="cloudDiagnosisField(item, 'root_cause')"><b>根因</b>{{ cloudDiagnosisField(item, 'root_cause') }}</p>
            <p v-if="cloudDiagnosisField(item, 'recommendation')"><b>处置建议</b>{{ cloudDiagnosisField(item, 'recommendation') }}</p>
            <p v-if="cloudDiagnosisField(item, 'explanation')"><b>诊断解释</b>{{ cloudDiagnosisField(item, 'explanation') }}</p>
          </div>
          <div class="cloud-feedback-meta">
            <span>Event {{ item.event_id }}</span>
            <span>置信度 {{ percent(item.cloud_confidence || 0) }}%</span>
            <span>Token {{ item.token_usage || 0 }}</span>
          </div>
        </div>
      </div>
      <a-empty v-else :image="simpleImage" description="暂无云端诊断结果" />
    </a-card>

    <a-card :bordered="false" class="panel events-panel">
      <template #title>
        <div class="panel-title">
          <span>实时事件流</span>
          <div class="event-actions">
            <span class="event-count">最近 {{ events.length }} 条</span>
            <a-tag v-if="pausedEvents.length" color="orange">已暂停 {{ pausedEvents.length }} 条新事件</a-tag>
            <a-button size="small" @click="togglePause">{{ streamPaused ? '继续刷新' : '暂停刷新' }}</a-button>
            <a-button v-if="pausedEvents.length" size="small" type="primary" @click="flushPausedEvents">载入新事件</a-button>
          </div>
        </div>
      </template>
      <a-table :dataSource="events" :columns="columns" :pagination="false" :rowKey="(row: InferredEvent) => row.event_id" size="middle" :scroll="{ y: 430 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'inferred_at'">{{ timeLabel(record.inferred_at) }}</template>
          <template v-else-if="column.dataIndex === 'edge_pred'">
            <a-tag :color="predictionColor(record.edge_pred)">{{ record.edge_pred }}</a-tag>
          </template>
          <template v-else-if="column.dataIndex === 'severity'">
            <span :class="severityClass(record.severity)">{{ record.severity }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'confidence'">{{ percent(record.confidence) }}%</template>
          <template v-else-if="column.dataIndex === 'inference_path'">
            <a-tag color="blue">{{ record.inference_path }}</a-tag>
          </template>
          <template v-else-if="column.dataIndex === 'route_action'">
            <a-tag :color="routeActionColor(routeDecision(record)?.action)">{{ routeDecision(record)?.action || '-' }}</a-tag>
          </template>
          <template v-else-if="column.dataIndex === 'raw_log'">
            <a-tooltip :title="logTooltip(record)"><span class="log-text">{{ record.raw_log || record.template_text }}</span></a-tooltip>
          </template>
          <template v-else-if="column.dataIndex === 'diagnosis'">
            <span class="diagnosis-preview">{{ diagnosisPreview(record) }}</span>
          </template>
        </template>
        <template #expandedRowRender="{ record }">
          <div class="event-detail">
            <div class="detail-block wide">
              <span class="detail-label">原始日志</span>
              <p>{{ record.raw_log || '-' }}</p>
            </div>
            <div class="detail-block wide">
              <span class="detail-label">日志模板</span>
              <p>{{ record.template_text }}</p>
            </div>
            <div class="detail-grid">
              <div class="detail-block">
                <span class="detail-label">诊断建议</span>
                <p>{{ record.advice || '-' }}</p>
              </div>
              <div class="detail-block">
                <span class="detail-label">建议编码</span>
                <p>{{ record.advice_code || '-' }}</p>
              </div>
              <div class="detail-block">
                <span class="detail-label">证据</span>
                <div class="evidence-list" v-if="record.evidence?.length">
                  <a-tag v-for="item in record.evidence" :key="item" color="purple">{{ item }}</a-tag>
                </div>
                <p v-else>-</p>
              </div>
              <div class="detail-block">
                <span class="detail-label">模型版本</span>
                <p>{{ record.model_version || '-' }}</p>
              </div>
              <div class="detail-block">
                <span class="detail-label">阶段耗时</span>
                <p>{{ latencyDetail(record) }}</p>
              </div>
              <div class="detail-block">
                <span class="detail-label">RAISE 路由</span>
                <p>{{ routeDetail(record) }}</p>
              </div>
              <div v-if="cloudFeedback(record)" class="detail-block wide cloud-diagnosis">
                <span class="detail-label">云端结构化诊断</span>
                <div class="cloud-diagnosis-header">
                  <a-tag color="geekblue">{{ cloudFeedback(record)?.cloud_pred || 'UNKNOWN' }}</a-tag>
                  <span>置信度 {{ percent(cloudFeedback(record)?.cloud_confidence || 0) }}%</span>
                  <span>{{ cloudFeedback(record)?.cloud_model }}</span>
                  <span>{{ cloudFeedback(record)?.cloud_latency_ms }} ms</span>
                </div>
                <p v-if="cloudDiagnosisField(cloudFeedback(record)!, 'root_cause')"><b>根因：</b>{{ cloudDiagnosisField(cloudFeedback(record)!, 'root_cause') }}</p>
                <p v-if="cloudDiagnosisField(cloudFeedback(record)!, 'recommendation')"><b>建议：</b>{{ cloudDiagnosisField(cloudFeedback(record)!, 'recommendation') }}</p>
                <p v-if="cloudDiagnosisField(cloudFeedback(record)!, 'explanation')"><b>解释：</b>{{ cloudDiagnosisField(cloudFeedback(record)!, 'explanation') }}</p>
              </div>
            </div>
          </div>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { TableColumnsType } from 'ant-design-vue';
import { Empty, message } from 'ant-design-vue';
import { HomeOutlined } from '@ant-design/icons-vue';
import type { EChartsCoreOption } from 'echarts/core';
import MetricCard from '../components/dashboard/MetricCard.vue';
import EChartPanel from '../components/dashboard/EChartPanel.vue';
import {
  getAbnormalAnalysis, getCompetitionMetrics, getDashboardSummary, getDashboardTimeseries, getRouterState, getRouterSummary,
  listCloudFeedback, listEvents, listRouterDecisions, listRouterOutbox, subscribeEvents, subscribeRouterEvents,
  type AbnormalAnalysis, type AbnormalCluster, type CloudFeedback, type CloudOutboxItem, type DashboardSummary,
  type CompetitionMetrics, type InferredEvent, type RouterDecision, type RouterStateResponse, type RouterSummary, type TimeseriesPoint
} from '../api/dashboard';
import { createTemplateFromCluster, type EdgePred } from '../api/edgeKnowledge';

const emptySummary: DashboardSummary = {
  raw_total: 0, inferred_total: 0, pending: 0, dlq_total: 0,
  ingest_rate_1m: 0, infer_rate_1m: 0, p95_latency_ms: 0,
  abnormal_rate: 0, fallback_rate: 0, classification: {}, paths: {}, generated_at: ''
};
const emptyRouterSummary: RouterSummary = {
  routed_total: 0,
  local_total: 0,
  cloud_total: 0,
  defer_total: 0,
  local_ratio: 0,
  cloud_ratio: 0,
  defer_ratio: 0,
  outbox_pending: 0,
  outbox_retry: 0,
  outbox_dlq: 0,
  p95_route_latency_ms: 0,
  average_score: 0,
  average_threshold: 0,
  last_action: ''
};
const emptyAnalysis: AbnormalAnalysis = {
  risk_level: 'LOW',
  abnormal_1m: 0,
  abnormal_5m: 0,
  growth_ratio: 0,
  top_types: {},
  top_clusters: [],
  priority_queue: [],
  generated_at: ''
};
const emptyCompetitionMetrics: CompetitionMetrics = {
  schema_version: 'competition-metrics-v1', window: '', collected_at: '',
  realtime: { end_to_end: { count: 0, avg_ms: 0, p50_ms: 0, p95_ms: 0 }, edge_processing: { count: 0, avg_ms: 0, p50_ms: 0, p95_ms: 0 }, by_path: {}, cloud_average_ms: 0 },
  effect: { ground_truth_evaluation: { status: 'not_available', sample_count: 0 }, cloud_comparison: { status: 'not_available', sample_count: 0 }, edge_cloud_agreement_rate: 0, edge_cloud_conflict_rate: 0, edge_correction_rate: 0, actionable_gain_rate: 0, average_reward: 0 },
  efficiency: { routed_total: 0, local_total: 0, cloud_total: 0, cloud_offload_rate: 0, estimated_upload_reduction_rate: 0, recent_input_bytes: 0, estimated_avoided_upload_bytes: 0, cloud_token_total: 0, cloud_cost_total: 0 },
  scalability: { distinct_sources: 0, distinct_equipment: 0, observed_inference_paths: 0, scenario_evidence: { status: 'not_available', sample_count: 0 } },
  stability: { completion_rate: 0, fallback_rate: 0, dlq_total: 0, outbox_success_rate: 0, outbox_retry: 0, outbox_dlq: 0, network_experiment: { status: 'not_available', sample_count: 0 } },
  consistency: { paired_samples: 0, conflict_count: 0, conflict_rate: 0, resolved_count: 0, resolution_success_rate: 0, multi_node_experiment: { status: 'not_available', sample_count: 0 } },
  evidence: { mechanisms: [], application_signals: [] }
};

interface RouterSystemState {
  CloudHealth?: string;
  RTTMS?: number;
  FailRate?: number;
  RateLimitRemaining?: number;
  MinuteBudgetRemaining?: number;
  HourBudgetRemaining?: number;
  DayBudgetRemaining?: number;
  CloudQueueLag?: number;
  OutboxLag?: number;
  QueueMax?: number;
  RecentCloudRatio?: number;
  MinCloudRatioForAudit?: number;
  EstimatedLatencyMS?: number;
  PrivacyCost?: number;
}

const summary = ref<DashboardSummary>({ ...emptySummary });
const routerSummary = ref<RouterSummary>({ ...emptyRouterSummary });
const routerDegraded = ref(true);
const routerDecisions = ref<RouterDecision[]>([]);
const outboxItems = ref<CloudOutboxItem[]>([]);
const cloudFeedbackItems = ref<CloudFeedback[]>([]);
const routerState = ref<RouterStateResponse>({ available: false });
const analysis = ref<AbnormalAnalysis>({ ...emptyAnalysis });
const competitionMetrics = ref<CompetitionMetrics>({ ...emptyCompetitionMetrics });
const router = useRouter();
const timeseries = ref<TimeseriesPoint[]>([]);
const events = ref<InferredEvent[]>([]);
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const streamConnected = ref(false);
const streamPaused = ref(false);
const pausedEvents = ref<InferredEvent[]>([]);
const depositingCluster = ref('');
let eventSource: EventSource | undefined;
let routerEventSource: EventSource | undefined;
let refreshTimer: number | undefined;
let eventFlushTimer: number | undefined;
let pendingEvents: InferredEvent[] = [];

const columns: TableColumnsType = [
  { title: '时间', dataIndex: 'inferred_at', width: 100 },
  { title: '设备', dataIndex: 'equipment', width: 140 },
  { title: '原始日志', dataIndex: 'raw_log', ellipsis: true },
  { title: '分类', dataIndex: 'edge_pred', width: 130 },
  { title: '严重度', dataIndex: 'severity', width: 80 },
  { title: '置信度', dataIndex: 'confidence', width: 90 },
  { title: '路径', dataIndex: 'inference_path', width: 100 },
  { title: '路由', dataIndex: 'route_action', width: 90 },
  { title: '诊断结果', dataIndex: 'diagnosis', width: 220 },
  { title: '时延', dataIndex: 'latency_ms', width: 90, customRender: ({ text }) => `${text} ms` }
];

const riskReason = computed(() => {
  if (analysis.value.risk_level === 'HIGH') return '异常速率、增长倍数或严重度已达到高风险阈值';
  if (analysis.value.risk_level === 'MEDIUM') return '存在持续异常聚集，建议关注优先处理队列';
  return '异常处于低位，暂无明显聚集风险';
});

const trendOption = computed<EChartsCoreOption>(() => ({
  color: ['#1677ff', '#fa8c16'],
  tooltip: { trigger: 'axis', axisPointer: { type: 'line' } },
  legend: {
    top: 0,
    right: 8,
    itemWidth: 10,
    itemHeight: 10,
    textStyle: { color: '#52677b' }
  },
  grid: { left: 12, right: 14, top: 42, bottom: 10, containLabel: true },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: timeseries.value.map((item) => minuteLabel(item.time)),
    axisLine: { lineStyle: { color: '#dbeafe' } },
    axisTick: { show: false },
    axisLabel: { color: '#789' }
  },
  yAxis: {
    type: 'value',
    minInterval: 1,
    splitLine: { lineStyle: { color: '#eef5fb' } },
    axisLabel: { color: '#789' }
  },
  series: [
    {
      name: '完成推理',
      type: 'line',
      smooth: true,
      showSymbol: false,
      symbol: 'circle',
      symbolSize: 6,
      emphasis: { focus: 'series' },
      lineStyle: { width: 3 },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(22, 119, 255, .28)' },
            { offset: 1, color: 'rgba(22, 119, 255, 0)' }
          ]
        }
      },
      data: timeseries.value.map((item) => item.inferred)
    },
    {
      name: '异常事件',
      type: 'line',
      smooth: true,
      showSymbol: false,
      symbol: 'circle',
      symbolSize: 6,
      emphasis: { focus: 'series' },
      lineStyle: { width: 3 },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(250, 140, 22, .3)' },
            { offset: 1, color: 'rgba(250, 140, 22, 0)' }
          ]
        }
      },
      data: timeseries.value.map((item) => item.abnormal)
    }
  ]
}));

const pathOption = computed<EChartsCoreOption>(() => horizontalBarOption(summary.value.paths, ['#1677ff', '#13c2c2', '#722ed1', '#fa8c16', '#8c8c8c'], '暂无路径数据'));
const classificationOption = computed<EChartsCoreOption>(() => horizontalBarOption(summary.value.classification, ['#22c55e', '#fa8c16', '#ef4444', '#1677ff', '#13c2c2', '#722ed1', '#8c8c8c'], '暂无分类数据'));
const routerActionOption = computed<EChartsCoreOption>(() => horizontalBarOption({
  LOCAL: routerSummary.value.local_total,
  CLOUD: routerSummary.value.cloud_total,
  DEFER: routerSummary.value.defer_total
}, ['#22c55e', '#1677ff', '#fa8c16'], '暂无路由数据'));
const routerDecisionMap = computed(() => {
  const map = new Map<string, RouterDecision>();
  for (const item of routerDecisions.value) {
    map.set(item.event_id, item);
  }
  return map;
});
const cloudFeedbackMap = computed(() => {
  const map = new Map<string, CloudFeedback>();
  for (const item of cloudFeedbackItems.value) map.set(item.event_id, item);
  return map;
});
const visibleCloudFeedback = computed(() => cloudFeedbackItems.value.slice(0, 12));
const systemState = computed<RouterSystemState>(() => {
  if (!routerState.value.available) return {};
  const payload = routerState.value.payload as { state?: RouterSystemState } | undefined;
  return payload?.state || {};
});

async function refresh() {
  try {
    const [nextSummary, nextTimeseries, nextAnalysis, nextRouter, nextDecisions, nextOutbox, nextState, nextCloudFeedback, nextCompetitionMetrics] = await Promise.all([
      getDashboardSummary(),
      getDashboardTimeseries(),
      getAbnormalAnalysis(),
      getRouterSummary(),
      listRouterDecisions(80),
      listRouterOutbox(20),
      getRouterState(),
      listCloudFeedback(80),
      getCompetitionMetrics()
    ]);
    summary.value = nextSummary;
    timeseries.value = nextTimeseries;
    analysis.value = nextAnalysis;
    routerSummary.value = nextRouter.summary || { ...emptyRouterSummary };
    routerDegraded.value = nextRouter.degraded;
    routerDecisions.value = nextDecisions;
    outboxItems.value = nextOutbox;
    routerState.value = nextState;
    cloudFeedbackItems.value = nextCloudFeedback;
    competitionMetrics.value = nextCompetitionMetrics;
  } catch (error) {
    console.error('refresh dashboard failed', error);
  }
}

onMounted(async () => {
  await Promise.all([refresh(), listEvents().then((items) => { events.value = items; })]);
  eventSource = subscribeEvents((item) => {
    streamConnected.value = true;
    pendingEvents.push(item);
  });
  eventSource.onopen = () => { streamConnected.value = true; };
  eventSource.onerror = () => { streamConnected.value = false; };
  routerEventSource = subscribeRouterEvents((item) => {
    const decision = item as RouterDecision;
    if (!decision.event_id) return;
    routerDecisions.value = [decision, ...routerDecisions.value.filter((old) => old.event_id !== decision.event_id)].slice(0, 80);
  });
  eventFlushTimer = window.setInterval(flushEvents, 500);
  refreshTimer = window.setInterval(refresh, 10000);
});

onBeforeUnmount(() => {
  eventSource?.close();
  routerEventSource?.close();
  if (refreshTimer) window.clearInterval(refreshTimer);
  if (eventFlushTimer) window.clearInterval(eventFlushTimer);
});

function flushEvents() {
  if (!pendingEvents.length) return;
  const incoming = pendingEvents.splice(0).reverse();
  if (streamPaused.value) {
    pausedEvents.value = [...incoming, ...pausedEvents.value].slice(0, 200);
    return;
  }
  mergeEvents(incoming);
}

function mergeEvents(incoming: InferredEvent[]) {
  const seen = new Set<string>();
  events.value = [...incoming, ...events.value]
    .filter((item) => !seen.has(item.event_id) && seen.add(item.event_id))
    .slice(0, 50);
}

function togglePause() {
  streamPaused.value = !streamPaused.value;
  if (!streamPaused.value) {
    flushPausedEvents();
  }
}

function flushPausedEvents() {
  if (!pausedEvents.value.length) return;
  const incoming = pausedEvents.value.splice(0);
  mergeEvents(incoming);
}

function goManage() {
  router.push('/manage');
}

function goHome() {
  router.push({ name: 'home' });
}

async function depositTemplate(item: AbnormalCluster) {
  if (!item.template_text) {
    message.warning('该聚合缺少日志模板，暂时不能沉淀');
    return;
  }
  depositingCluster.value = item.cluster_id;
  try {
    const result = await createTemplateFromCluster({
      template_text: item.template_text,
      sample_raw_log: item.sample_raw_log,
      source: 'abnormal_cluster',
      edge_pred: item.edge_pred as EdgePred,
      max_severity: item.max_severity,
      avg_confidence: item.avg_confidence,
      evidence: item.evidence || [],
      advice_code: item.advice_code || '',
      advice: item.advice || '',
      status: 'draft'
    });
    if (result.created) {
      message.success('已沉淀为模板草稿，请到模板知识库审核后发布');
    } else {
      message.info('模板知识库中已存在该模板');
    }
  } catch (error) {
    console.error('deposit template failed', error);
    message.error('沉淀模板失败，请稍后重试');
  } finally {
    depositingCluster.value = '';
  }
}

function percent(value: number) { return Math.round(value * 100); }
function optionalPercent(value?: number) { return value === undefined || !Number.isFinite(value) ? '-' : `${value.toFixed(1)}%`; }
function byteText(value: number) {
  if (!value) return '0 B';
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MiB`;
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${value} B`;
}
function numberText(value: number | undefined) {
  if (value === undefined || value === null || Number.isNaN(value)) return '-';
  if (Math.abs(value) >= 100) return `${Math.round(value)}`;
  return value.toFixed(1);
}
function cloudHealthColor(value?: string) {
  if (value === 'UP') return 'green';
  if (value === 'DOWN') return 'red';
  return 'default';
}
function timeLabel(value: string) { return value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false }) : '-'; }
function minuteLabel(value: string) { return value ? new Date(value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : ''; }
function relativeTime(value: string) {
  if (!value) return '-';
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 60) return `${seconds} 秒前`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  return `${hours} 小时前`;
}
function riskText(value: AbnormalAnalysis['risk_level']) {
  if (value === 'HIGH') return '高';
  if (value === 'MEDIUM') return '中';
  return '低';
}
function ratioText(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-';
  return `${value.toFixed(1)}x`;
}
function evidenceText(item: AbnormalCluster) {
  return item.evidence?.filter(Boolean).slice(0, 3).join('；') || '';
}
function equipmentText(item: AbnormalCluster) {
  const names = item.equipments?.filter(Boolean) || [];
  if (!names.length) return `${item.affected_equipments || 0} 台`;
  const shown = names.slice(0, 3).join('、');
  const rest = names.length > 3 ? ` 等 ${names.length} 台` : '';
  return shown + rest;
}
function severityClass(value: number) { return value >= 90 ? 'severity critical' : value >= 70 ? 'severity high' : 'severity'; }
function diagnosisPreview(record: InferredEvent) {
  const cloud = cloudFeedback(record);
  if (cloud) return cloud.recommendation || cloud.root_cause || cloud.explanation || `云端诊断：${cloud.cloud_pred}`;
  if (record.advice) return record.advice;
  if (record.evidence?.length) return record.evidence.join('；');
  if (record.inference_path === 'EDGE_LLM') return '小模型已完成细粒度诊断';
  if (record.inference_path === 'MODEL') return '分类模型初筛结果';
  return '-';
}
function cloudFeedback(record: InferredEvent) {
  return cloudFeedbackMap.value.get(record.event_id);
}
function cloudDiagnosisField(item: CloudFeedback, field: 'root_cause' | 'recommendation' | 'explanation'): string {
  const direct = item[field]?.trim();
  if (direct && (field !== 'explanation' || !direct.trimStart().startsWith('{'))) return direct;
  const raw = item.explanation?.trim() || '';
  if (raw) {
    const cleaned = raw.replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
    try {
      const parsed = JSON.parse(cleaned) as Record<string, unknown>;
      const value = parsed[field];
      if (typeof value === 'string' && value.trim()) return value.trim();
    } catch {
      const match = cleaned.match(new RegExp(`"${field}"\\s*:\\s*"([^"\\n]*)`, 'i'));
      if (match?.[1]) return match[1];
    }
    if (field === 'explanation' && !cleaned.startsWith('{') && !cleaned.startsWith('[')) return raw;
  }
  return '';
}
function logTooltip(record: InferredEvent) {
  const raw = record.raw_log || '-';
  return `原始日志：${raw}\n\n日志模板：${record.template_text}`;
}
function latencyDetail(record: InferredEvent) {
  const stage = record.stage_latency_ms || {};
  return [
    `normalize=${stage.normalize ?? 0}ms`,
    `cache=${stage.cache ?? 0}ms`,
    `model=${stage.model ?? 0}ms`,
    `edge_llm=${stage.edge_llm ?? 0}ms`,
    `publish=${stage.publish ?? 0}ms`
  ].join(' / ');
}
function routeDecision(record: InferredEvent) {
  return routerDecisionMap.value.get(record.event_id);
}
function routeDetail(record: InferredEvent) {
  const decision = routeDecision(record);
  if (!decision) return '暂无路由决策';
  return [
    `action=${decision.action}`,
    `score=${scoreText(decision.score)}`,
    `threshold=${scoreText(decision.threshold)}`,
    `reason=${decision.reason || '-'}`,
    `cloud=${decision.cloud_health || '-'}`
  ].join(' / ');
}
function scoreText(value: number | undefined) {
  if (value === undefined || value === null || Number.isNaN(value)) return '-';
  return value.toFixed(2);
}
function routeActionColor(value?: string) {
  if (value === 'LOCAL') return 'green';
  if (value === 'CLOUD') return 'blue';
  if (value === 'DEFER') return 'orange';
  return 'default';
}
function outboxStatusColor(value: string) {
  if (value === 'SENT' || value === 'DONE') return 'green';
  if (value === 'DLQ') return 'red';
  if (value === 'RETRY') return 'orange';
  if (value === 'SENDING') return 'blue';
  return 'default';
}
function predictionColor(value: string) {
  if (value === 'NORMAL') return 'green';
  if (value === 'UNKNOWN') return 'default';
  if (value === 'SECURITY' || value === 'DISK_ERROR') return 'red';
  return 'orange';
}

function chartEntries(items: Record<string, number>) {
  return Object.entries(items)
    .filter(([, value]) => value > 0)
    .sort((a, b) => b[1] - a[1])
    .map(([name, value]) => ({ name, value }));
}

function horizontalBarOption(items: Record<string, number>, colors: string[], emptyText: string): EChartsCoreOption {
  const data = chartEntries(items);
  const names = data.map((item) => item.name).reverse();
  const values = data.map((item) => item.value).reverse();
  return {
    color: colors,
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 10, right: 28, top: 12, bottom: 10, containLabel: true },
    title: data.length ? undefined : {
      text: emptyText,
      left: 'center',
      top: '42%',
      textStyle: { color: '#9aaabe', fontSize: 13, fontWeight: 400 }
    },
    xAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#eef5fb' } },
      axisLabel: { color: '#789' }
    },
    yAxis: {
      type: 'category',
      data: names,
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: { color: '#52677b', fontSize: 11 }
    },
    series: [
      {
        type: 'bar',
        barMaxWidth: 16,
        data: values.map((value, index) => ({ value, itemStyle: { color: colors[index % colors.length] } })),
        label: { show: true, position: 'right', color: '#16324f', fontWeight: 700 },
        itemStyle: {
          borderRadius: [0, 8, 8, 0],
          shadowBlur: 10,
          shadowColor: 'rgba(22, 119, 255, .12)'
        }
      }
    ]
  };
}
</script>

<style scoped>
.dashboard { min-height: 100vh; box-sizing: border-box; padding: 28px max(24px, calc((100vw - 1560px) / 2)); color: #102a43; background: radial-gradient(circle at 12% 0%, rgba(22, 119, 255, .15), transparent 28%), radial-gradient(circle at 88% 12%, rgba(19, 194, 194, .12), transparent 26%), linear-gradient(180deg, #f5fbff 0%, #eaf4ff 100%); }
.page-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 30px; }
.product-eyebrow { display: block; margin-bottom: 7px; color: #1677ff; font-size: 11px; font-weight: 800; letter-spacing: .13em; text-transform: uppercase; }
.page-head h1 { margin: 0; color: #0f2a44; font-size: 28px; letter-spacing: -.02em; }
.page-head p { margin: 5px 0 0; color: #6b7f93; }
.connection { display: flex; align-items: center; gap: 14px; }
.nav-icon-btn { padding: 0; width: 36px; height: 32px; border-radius: 999px; color: rgba(15, 42, 74, .85); border: 1px solid rgba(24, 144, 255, .18); background: rgba(255, 255, 255, .72); display: inline-flex; align-items: center; justify-content: center; }
.nav-icon-btn:hover { background: rgba(24, 144, 255, .08); border-color: rgba(24, 144, 255, .35); }
.stream-state { background: white; border: 1px solid #dbeafe; border-radius: 20px; padding: 8px 14px; font-size: 13px; }
.pulse { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 7px; }
.online { background: #22c55e; box-shadow: 0 0 0 5px rgba(34, 197, 94, .12); }
.offline { background: #f59e0b; }
.section-head { display: flex; justify-content: space-between; align-items: center; margin: 0 2px 14px; }
.section-head > div { display: flex; align-items: center; gap: 11px; }
.section-head > div > span { display: flex; width: 30px; height: 30px; align-items: center; justify-content: center; border-radius: 9px; color: #fff; background: linear-gradient(135deg, #1677ff, #13c2c2); box-shadow: 0 7px 16px rgba(22, 119, 255, .2); font-size: 12px; font-weight: 800; }
.section-head b, .section-head small { display: block; }
.section-head b { color: #16324f; font-size: 16px; }
.section-head small, .section-status { margin-top: 2px; color: #829ab1; font-size: 12px; }
.content-section-head { margin-top: 28px; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(150px, 1fr)); gap: 14px; }
.trend-panel { margin-top: 0; }
.competition-panel { margin-top: 14px; border-color: rgba(114, 46, 209, .18); }
.panel-title > div { display: grid; gap: 2px; }
.panel-kicker { color: #7c3aed; font-size: 10px; font-weight: 800; letter-spacing: .11em; }
.competition-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.competition-group { min-width: 0; padding: 14px; border: 1px solid #e9d5ff; border-radius: 14px; background: linear-gradient(145deg, #faf5ff, #fff); }
.competition-group h3 { margin: 0 0 10px; color: #5b21b6; font-size: 14px; }
.competition-group small { display: block; margin-top: 9px; color: #829ab1; line-height: 1.45; }
.competition-kv { display: flex; justify-content: space-between; gap: 10px; padding: 6px 0; color: #52677b; font-size: 13px; }
.competition-kv b { color: #16324f; white-space: nowrap; }
.chart-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.router-grid { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(360px, .85fr); gap: 14px; }
.panel { overflow: hidden; border: 1px solid rgba(24, 144, 255, .12); border-radius: 16px; background: rgba(255, 255, 255, .92); backdrop-filter: blur(12px); box-shadow: 0 12px 32px rgba(15, 42, 74, .065); }
.chart-panel { position: relative; }
.chart-panel::before { content: ""; position: absolute; inset: 0; pointer-events: none; background: linear-gradient(135deg, rgba(22, 119, 255, .08), transparent 38%); }
.trend-chart { height: 286px; }
.router-panel { border-color: rgba(19, 194, 194, .2); }
.cloud-feedback-panel { margin-top: 14px; border-color: rgba(114, 46, 209, .2); }
.cloud-feedback-list { display: grid; gap: 12px; }
.cloud-feedback-item { padding: 14px 16px; border: 1px solid #ede9fe; border-radius: 14px; background: linear-gradient(135deg, #faf5ff, #fff); }
.cloud-feedback-head, .cloud-feedback-head > div, .cloud-feedback-meta { display: flex; align-items: center; gap: 10px; }
.cloud-feedback-head { justify-content: space-between; color: #789; font-size: 12px; }
.cloud-feedback-head b { color: #243b53; font-size: 14px; }
.cloud-feedback-content { display: grid; gap: 7px; margin-top: 12px; }
.cloud-feedback-content p { margin: 0; color: #486581; line-height: 1.65; white-space: pre-wrap; overflow-wrap: anywhere; }
.cloud-feedback-content b { display: inline-block; width: 72px; color: #5b21b6; }
.cloud-feedback-meta { flex-wrap: wrap; margin-top: 10px; color: #829ab1; font-size: 12px; }
.router-summary { display: grid; grid-template-columns: minmax(0, 1fr) 220px; gap: 14px; align-items: stretch; }
.router-chart { height: 220px; }
.router-explain { display: grid; grid-template-columns: 1fr; gap: 10px; }
.router-kv { min-width: 0; padding: 12px; border: 1px solid #dbeafe; border-radius: 12px; background: #f8fbff; }
.router-kv span { display: block; color: #789; font-size: 12px; }
.router-kv b { display: block; margin-top: 5px; color: #16324f; font-size: 18px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.router-state-preview { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: 12px; }
.state-chip { min-width: 0; padding: 10px 12px; border: 1px solid #ccfbf1; border-radius: 12px; background: #f0fdfa; }
.state-chip span { display: block; margin-bottom: 6px; color: #0f766e; font-size: 12px; font-weight: 700; }
.state-chip b { display: block; color: #134e4a; font-size: 16px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.outbox-list { display: grid; gap: 10px; max-height: 306px; overflow: auto; padding-right: 2px; }
.outbox-item { min-width: 0; padding: 12px; border: 1px solid #dbeafe; border-radius: 12px; background: #f8fbff; }
.outbox-main { display: flex; align-items: center; gap: 8px; min-width: 0; color: #16324f; }
.outbox-main b { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.outbox-main span { flex: 0 0 auto; color: #789; font-size: 12px; }
.outbox-meta { display: flex; gap: 10px; margin-top: 7px; color: #789; font-size: 12px; }
.outbox-meta span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.abnormal-panel { border-color: rgba(250, 140, 22, .24); }
.analysis-summary { display: grid; grid-template-columns: 1.4fr repeat(3, 1fr); gap: 12px; margin-bottom: 16px; }
.risk-card, .analysis-stat { border-radius: 14px; padding: 14px; background: #f8fbff; border: 1px solid #dbeafe; }
.risk-card { color: white; background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%); }
.risk-card.medium { background: linear-gradient(135deg, #f59e0b 0%, #f97316 100%); }
.risk-card.high { background: linear-gradient(135deg, #ef4444 0%, #b91c1c 100%); }
.risk-card span, .analysis-stat span { display: block; font-size: 12px; opacity: .82; }
.risk-card b, .analysis-stat b { display: block; margin-top: 5px; font-size: 26px; line-height: 1; }
.risk-card small { display: block; margin-top: 8px; opacity: .9; }
.analysis-stat b { color: #16324f; }
.analysis-grid { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr); gap: 16px; }
.analysis-grid > div { min-width: 0; }
.section-title { margin-bottom: 10px; color: #16324f; font-weight: 700; }
.cluster-list, .priority-list { display: grid; min-width: 0; gap: 10px; }
.cluster-card { min-width: 0; padding: 12px; border: 1px solid #fed7aa; border-left: 5px solid #fa8c16; border-radius: 12px; background: linear-gradient(180deg, #fffaf0 0%, #fff7ed 100%); }
.cluster-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; margin-bottom: 8px; }
.cluster-count { color: #9a5a00; font-size: 12px; font-weight: 700; }
.cluster-title { color: #16324f; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cluster-log { margin-top: 5px; color: #52677b; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cluster-meta { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; color: #789; font-size: 12px; }
.cluster-footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-top: 8px; }
.cluster-advice { min-width: 0; color: #6b4e16; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.priority-item { display: flex; min-width: 0; max-width: 100%; gap: 10px; padding: 12px; box-sizing: border-box; border: 1px solid #dbeafe; border-radius: 12px; background: #f8fbff; }
.priority-rank { flex: 0 0 36px; width: 36px; height: 36px; border-radius: 50%; display: flex; align-items: center; justify-content: center; background: #1677ff; color: white; font-weight: 800; }
.priority-body { min-width: 0; flex: 1; }
.priority-main { display: flex; justify-content: space-between; gap: 10px; color: #16324f; font-weight: 700; }
.priority-main span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.priority-main b { flex: 0 0 auto; }
.priority-desc { margin-top: 4px; color: #52677b; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.priority-actions { display: flex; justify-content: flex-end; margin-top: 6px; }
.events-panel { margin-top: 14px; }
.panel-title { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.event-actions { display: flex; align-items: center; gap: 8px; }
.event-count { color: #789; font-size: 12px; font-weight: 400; }
.log-text { display: block; max-width: 460px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.diagnosis-preview { display: block; max-width: 200px; color: #52677b; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.event-detail { background: #f8fbff; border: 1px solid #dbeafe; border-radius: 12px; padding: 14px; }
.detail-grid { display: grid; grid-template-columns: repeat(3, minmax(160px, 1fr)); gap: 12px; }
.detail-block { min-width: 0; }
.detail-block.wide { margin-bottom: 12px; }
.detail-label { display: block; color: #789; font-size: 12px; margin-bottom: 5px; }
.detail-block p { margin: 0; color: #16324f; white-space: pre-wrap; word-break: break-word; }
.cloud-diagnosis { padding: 12px; border: 1px solid #adc6ff; border-radius: 10px; background: #f0f5ff; }
.cloud-diagnosis-header { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 8px; color: #52677b; font-size: 12px; }
.cloud-diagnosis p + p { margin-top: 6px; }
.evidence-list { display: flex; flex-wrap: wrap; gap: 6px; }
.severity { font-weight: 700; color: #1677ff; }
.severity.high { color: #fa8c16; }
.severity.critical { color: #ef4444; }
@media (max-width: 1200px) { .metrics, .competition-grid { grid-template-columns: repeat(2, 1fr); } .chart-grid, .router-grid, .analysis-grid { grid-template-columns: 1fr; } .analysis-summary { grid-template-columns: repeat(2, 1fr); } .detail-grid { grid-template-columns: 1fr; } }
@media (max-width: 860px) { .router-summary { grid-template-columns: 1fr; } .router-state-preview { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 720px) { .dashboard { padding: 16px; } .page-head { align-items: flex-start; gap: 14px; } .page-head h1 { font-size: 23px; } .connection { align-items: flex-end; flex-direction: column; } .section-status { display: none; } .router-state-preview, .competition-grid { grid-template-columns: 1fr; } }
</style>
