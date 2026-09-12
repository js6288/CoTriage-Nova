# router-controller 路由算法设计

## 1. 研究定位

`router-controller` 是 CoTriage 的云边协同决策核心。`edge-infer` 已经回答“这条日志是什么、严重程度如何、边缘判断有多可靠”；router 进一步回答“这条日志应当留在边缘、本次上云、还是暂缓补传”。如果只用固定阈值，例如 `confidence < 0.7 则上云`，系统很容易被审稿人认为是工程规则，难以支撑 TC、TSC、TPDS 这类 CCF-A 期刊。

本文将路由算法形式化为如下核心问题：

> 在云端预算、网络状态、时延 SLA、弱网自治、数据敏感性和运维策略约束下，如何对连续到达的工业日志事件做在线云边协同推理路由，使高风险和高不确定事件尽可能获得云端复核，同时最小化云端调用成本、排队时延和弱网丢失风险。

算法命名为：

```text
RAISE: Risk-Aware, uncertainty-calibrated, Intent-constrained, Self-adaptive Edge-cloud routing
```

中文称为“风险感知、不确定性校准、策略意图约束的自适应云边路由算法”。

RAISE 的论文价值不在于给出一个复杂公式，而在于把工业 AIOps 中真实存在的多个矛盾统一成在线约束优化问题，并给出可解释、可部署、可在弱网下保持最终一致性的算法框架。

## 2. 问题定义

### 2.1 输入事件

每个进入 router 的事件记为 `e_t`，由 `cotriage.events.inferred` 输入：

```text
e_t = {
  event_id,
  source,
  template_id,
  edge_pred,
  severity,
  confidence,
  uncertainty,
  inference_path,
  evidence,
  latency_ms
}
```

router 同时读取系统态：

```text
s_t = {
  network_rtt_ms,
  network_fail_rate,
  cloud_health,
  infer_queue_lag,
  cloud_queue_lag,
  outbox_lag,
  budget_remaining,
  rate_limit_remaining,
  recent_cloud_accuracy,
  recent_edge_disagreement
}
```

以及用户策略：

```text
p_t = {
  sla_ms,
  cloud_budget_per_minute,
  cloud_budget_per_day,
  privacy_level,
  force_cloud_rules,
  force_local_rules,
  deny_cloud_rules,
  min_cloud_ratio_for_audit
}
```

### 2.2 决策动作

router 输出三类动作：

```text
a_t in {LOCAL, CLOUD, DEFER}
```

- `LOCAL`：边缘结果可直接落库或触发告警。
- `CLOUD`：立即发送到 `cotriage.events.cloud.req`，由 server/cloud-gateway 调用云端大模型。
- `DEFER`：当前事件理论上值得上云，但网络、预算或限流不满足，先进入 `cloud_outbox`，待恢复后补传。

### 2.3 优化目标

对连续事件流，RAISE 优化以下目标：

```text
minimize  E[
  C_miss(e_t, a_t)
  + lambda_cost * C_cloud(a_t)
  + lambda_lat  * C_latency(e_t, a_t)
  + lambda_net  * C_network(a_t, s_t)
  + lambda_priv * C_privacy(e_t, a_t)
]
```

其中：

- `C_miss`：高风险或高不确定事件未上云导致的漏判风险；
- `C_cloud`：云端 token、API 调用、带宽成本；
- `C_latency`：路由与排队导致的 SLA 违约成本；
- `C_network`：弱网时立即上云失败的风险；
- `C_privacy`：敏感日志上云带来的合规成本。

约束条件：

```text
P(latency_t <= sla_ms) >= 1 - epsilon
cloud_calls_per_minute <= B_minute
cloud_calls_per_day <= B_day
privacy_policy(e_t, a_t) must be satisfied
DEFER events must be eventually replayed when cloud is healthy
```

这一定义可以成为论文里的核心建模：它比“阈值路由”更像系统领域的在线资源调度问题。

## 3. 最终算法公式

本节给出可以直接写入论文方法章节的公式化版本。RAISE 被定义为一个在线约束决策算法：每个时间步到达一个事件，算法观测边缘推理结果、系统状态和策略约束，输出 `LOCAL`、`CLOUD` 或 `DEFER`。以下公式即最终论文算法。

### 3.1 符号定义

令连续到达的日志事件流为：

```text
E = {e_1, e_2, ..., e_T}
```

对第 `t` 个事件，边缘推理器输出特征向量：

```text
x_t = [r_t, c_t, u_t, m_t, h_t, z_t]
```

其中：

- `r_t in [0,1]`：归一化严重度；
- `c_t in [0,1]`：边缘模型原始置信度；
- `u_t in [0,1]`：校准后的不确定性；
- `m_t`：推理路径编码，如 `RULE`、`CACHE`、`MODEL`、`EDGE_LLM`、`FALLBACK`；
- `h_t`：历史统计特征，如模板频率、边云分歧率；
- `z_t`：上下文特征，如受影响设备数、时间窗口内异常增长率。

系统状态记为：

```text
s_t = [q_t, b_t, n_t, l_t]
```

其中：

- `q_t`：云端队列和 outbox 积压压力；
- `b_t`：云端调用预算压力；
- `n_t`：网络压力；
- `l_t`：系统近期上云审计压力。

动作空间为：

```text
A = {a^L, a^C, a^D}
```

分别表示：

```text
a^L = LOCAL
a^C = CLOUD
a^D = DEFER
```

### 3.2 不确定性校准

原始置信度 `c_t` 往往存在过度自信问题，因此 RAISE 不直接使用 `1-c_t` 作为不确定性。给定温度参数 `T > 0`，可使用温度缩放得到校准概率：

```text
\hat{p}_t = sigma(logit(c_t) / T)
```

其中：

```text
sigma(x) = 1 / (1 + exp(-x))
logit(c_t) = log(c_t / (1 - c_t))
```

校准后的不确定性定义为：

```text
u_t = 1 - \hat{p}_t
```

更一般地，可将校准器写成：

```text
\hat{p}_t = g_phi(c_t, r_t, m_t, f_t)
u_t = 1 - \hat{p}_t
```

其中 `f_t` 为模板频率，`g_phi` 是在验证集上训练得到的校准函数。论文中可以报告 Expected Calibration Error：

```text
ECE = sum_{k=1}^{K} (|B_k| / N) * |acc(B_k) - conf(B_k)|
```

用于证明校准前后路由决策质量的变化。

### 3.3 上云价值函数

RAISE 先估计事件上云的内在价值。定义：

```text
v_t = w_r r_t + w_u u_t + w_n nvl_t + w_d d_t + w_f fli_t + w_m rho(m_t)
```

其中：

- `nvl_t`：模板新颖度；
- `d_t`：历史边云分歧率；
- `fli_t`：影响范围；
- `rho(m_t)`：推理路径风险函数；
- `w = [w_r, w_u, w_n, w_d, w_f, w_m]`：非负权重。

约束：

```text
w_i >= 0
sum_i w_i = 1
```

各分量定义如下。

模板新颖度：

```text
nvl_t = exp(- freq(template_t, W) / tau_f)
```

其中 `freq(template_t, W)` 表示模板在滑动窗口 `W` 内出现次数，`tau_f` 为平滑系数。新模板或低频模板的 `nvl_t` 更高。

历史边云分歧率：

```text
d_t = (M_t + alpha) / (N_t + alpha + beta)
```

其中 `N_t` 是同类模板历史上云次数，`M_t` 是云端纠正边缘结果的次数，`alpha` 和 `beta` 是 Beta 先验参数，用于冷启动平滑。

影响范围：

```text
fli_t = min(1, affected_devices(template_t, W) / D_max)
```

推理路径风险：

```text
rho(RULE) = 0.05
rho(CACHE) = 0.10
rho(MODEL) = 0.45
rho(EDGE_LLM) = 0.65
rho(FALLBACK) = 1.00
```

### 3.4 云端边际收益模型

上云价值 `v_t` 表示“是否值得看一眼”，但云端是否真的会纠正边缘结果需要另行估计。定义云端边际收益：

```text
g_t = P(y_t^C != y_t^E | x_t)
```

其中 `y_t^E` 是边缘预测，`y_t^C` 是云端预测或云端复核标签。

RAISE 使用在线收益模型估计云端复核的边际价值。默认采用在线逻辑回归：

```text
g_t = sigma(theta^T psi_t)
```

其中：

```text
psi_t = [r_t, u_t, nvl_t, d_t, fli_t, rho(m_t), 1]
```

云端结果返回后，构造反馈：

```text
R_t =
  1, if cloud corrects or enriches edge result
  0, otherwise
```

在线逻辑回归更新为：

```text
theta_{t+1} = theta_t + eta (R_t - g_t) psi_t
```

为避免系统只利用历史已知高收益事件而忽视新模板，RAISE 在收益估计中加入上置信界探索项：

```text
g_t^{ucb} = sigma(theta^T psi_t) + alpha_ucb * sqrt(psi_t^T A^{-1} psi_t)
```

并在收到反馈后更新：

```text
A <- A + psi_t psi_t^T
b <- b + R_t psi_t
theta <- A^{-1} b
```

最终决策分数使用截断后的探索收益：

```text
g_t = min(1, g_t^{ucb})
```

### 3.5 成本函数

RAISE 将上云成本拆成云调用成本、时延成本、网络风险和隐私成本：

```text
k_t = lambda_c k_t^C + lambda_l k_t^L + lambda_n k_t^N + lambda_p k_t^P
```

云调用成本：

```text
k_t^C = estimated_tokens(e_t) / token_budget_remaining_t
```

时延成本：

```text
k_t^L = max(0, estimated_latency_t - SLA) / SLA
```

网络风险：

```text
k_t^N = omega_r * norm(RTT_t) + omega_f * fail_rate_t + omega_h * 1[cloud_health_t = DOWN]
```

隐私成本：

```text
k_t^P =
  1, if privacy_level(e_t) forbids cloud
  p_sensitive, otherwise
```

其中 `lambda_*` 和 `omega_*` 为非负权重。

### 3.6 动态路由阈值

固定阈值无法适应故障爆发、预算收缩和弱网状态。RAISE 定义动态阈值：

```text
\Theta_t = \Theta_0
  + alpha_q Q_t
  + alpha_b B_t
  + alpha_n N_t
  - alpha_a A_t
```

其中：

```text
Q_t = min(1, (cloud_queue_lag_t + outbox_lag_t) / Q_max)
B_t = 1 - budget_remaining_t / budget_total_t
N_t = omega_r norm(RTT_t) + omega_f fail_rate_t + omega_h 1[cloud_health_t = DOWN]
A_t = max(0, audit_ratio_min - cloud_ratio_recent_t)
```

解释：

- `Q_t` 越大，云端越拥塞，阈值上升；
- `B_t` 越大，预算越紧，阈值上升；
- `N_t` 越大，网络越差，立即上云阈值上升；
- `A_t` 越大，近期审计样本不足，阈值下降，系统保留少量探索性上云。

### 3.7 动作评分与决策规则

定义事件的上云综合分数：

```text
S_t = beta_v v_t + beta_g g_t - beta_k k_t
```

若事件命中强策略约束，则策略优先：

```text
a_t =
  a^L, if e_t matches deny-cloud or force-local policy
  a^C, if e_t matches force-cloud policy and sendable(s_t)
  a^D, if e_t matches force-cloud policy and not sendable(s_t)
```

否则使用以下决策：

```text
a_t =
  a^L, if S_t < \Theta_t
  a^C, if S_t >= \Theta_t and sendable(s_t)
  a^D, if S_t >= \Theta_t and not sendable(s_t)
```

其中：

```text
sendable(s_t) =
  1[cloud_health_t = UP]
  * 1[rate_limit_remaining_t > 0]
  * 1[budget_remaining_t > 0]
  * 1[fail_rate_t <= delta_f]
```

### 3.8 Outbox 优先级

当 `a_t = a^D` 时，事件进入 outbox。补传优先级定义为：

```text
P_t^{replay} =
  gamma_r r_t
  + gamma_u u_t
  + gamma_g g_t
  + gamma_a age_t
  + gamma_f 1[force_cloud_t]
```

其中：

```text
age_t = min(1, waiting_time_t / deadline_t)
```

网络恢复后，outbox replayer 每次选择：

```text
e^* = argmax_{e_i in Outbox} P_i^{replay}
```

这使补传不是 FIFO，而是风险、收益和等待时间联合驱动。

### 3.9 约束优化视角

RAISE 可被表述为以下在线约束优化问题：

```text
max_{a_1,...,a_T} sum_{t=1}^{T}
  1[a_t in {a^C, a^D}] * (beta_v v_t + beta_g g_t)
  - 1[a_t = a^C] * beta_k k_t
```

subject to:

```text
sum_{t=1}^{T} 1[a_t = a^C] <= B_T
P(L_t <= SLA) >= 1 - epsilon
a_t respects privacy and operator policies
e_t with a_t = a^D is eventually replayed or sent to DLQ
```

由于完整问题需要未来事件信息，在线系统无法直接求全局最优。RAISE 使用动态阈值和在线收益学习构造可部署的近似策略：

```text
pi_RAISE(e_t, s_t, p_t) -> a_t
```

### 3.10 算法伪代码

论文中使用如下形式：

```text
Algorithm 1: RAISE Online Edge-Cloud Routing

Input:
  event stream E, system state stream S, policy P,
  calibrator g_phi, gain model theta,
  token bucket B, outbox O

Output:
  routing decisions a_t in {LOCAL, CLOUD, DEFER}

for each event e_t do
  x_t <- ExtractFeatures(e_t)

  if MatchDenyCloud(e_t, P) or MatchForceLocal(e_t, P) then
      a_t <- LOCAL
      EmitDecision(e_t, a_t)
      continue
  end if

  p_hat_t <- g_phi(x_t)
  u_t <- 1 - p_hat_t

  v_t <- CloudValue(x_t, u_t)
  g_t <- ExpectedGain(theta, x_t, u_t)
  k_t <- ExpectedCost(e_t, s_t)
  Theta_t <- DynamicThreshold(s_t, P)
  S_t <- beta_v v_t + beta_g g_t - beta_k k_t

  if MatchForceCloud(e_t, P) or S_t >= Theta_t then
      if Sendable(s_t, B) then
          ConsumeToken(B)
          a_t <- CLOUD
          PublishCloudRequest(e_t)
      else
          a_t <- DEFER
          InsertOutbox(O, e_t, ReplayPriority(e_t))
      end if
  else
      a_t <- LOCAL
  end if

  EmitDecision(e_t, a_t, S_t, Theta_t)
end for

Upon cloud result y_t^C for event e_t:
  R_t <- Reward(y_t^E, y_t^C)
  theta <- UpdateGainModel(theta, x_t, R_t)
```

### 3.11 可证明性质

预算可行性。若每个 `CLOUD` 动作必须先从 token bucket 消费一个 token，桶容量为 `C`，补充速率为 `rho`，则任意时间窗口 `[0,T]` 内：

```text
N_cloud(T) <= C + rho T
```

这保证云端调用数不会超过预算上界。

策略安全性。若策略匹配函数 `MatchDenyCloud(e_t, P)=true` 时算法直接返回 `LOCAL`，则对任意事件：

```text
MatchDenyCloud(e_t, P) => a_t != CLOUD
```

即隐私禁止上云事件不会被学习模型覆盖。

弱网最终补偿。若事件 `e_t` 满足 `a_t=DEFER`，且在未来存在时间 `tau > t` 使得 `sendable(s_tau)=true`，并且 outbox replayer 持续运行，则 `e_t` 最终进入 `CLOUD` 或在超过最大重试后进入 `DLQ`：

```text
a_t = DEFER => eventually(CLOUD(e_t) or DLQ(e_t))
```

该性质来自 outbox 状态机和周期性重放机制。

### 3.12 核心公式摘要

RAISE 的核心可摘要为三行：

```text
S_t = beta_v V(e_t) + beta_g G(e_t) - beta_k K(e_t, s_t)
Theta_t = Theta_0 + alpha_q Q_t + alpha_b B_t + alpha_n N_t - alpha_a A_t
a_t = LOCAL if S_t < Theta_t, CLOUD if S_t >= Theta_t and sendable(s_t), otherwise DEFER
```

再解释：

- `V(e_t)` 衡量风险和不确定性；
- `G(e_t)` 衡量云端可能带来的边际收益；
- `K(e_t, s_t)` 衡量成本、时延、网络和隐私代价；
- `Theta_t` 根据系统压力自适应调整；
- `DEFER` 使弱网环境下的上云需求保持最终一致性。

## 4. 最终算法模块

RAISE 最终版由六个模块组成，所有模块均参与论文主算法，不作为可选增强项：

1. 风险-不确定性价值估计；
2. 云端收益预测；
3. 在线探索与反馈更新；
4. 策略意图约束；
5. 在线预算与队列背压控制；
6. 弱网自治与最终一致性补偿。

### 4.1 风险-不确定性价值估计

首先计算事件的上云价值 `V_cloud(e_t)`。直觉是：严重、低置信、新模板、影响范围大、历史上边云分歧高的事件更值得上云。

```text
R_t = normalize(severity_t)
U_t = calibrated_uncertainty_t
N_t = novelty(template_id_t)
D_t = edge_cloud_disagreement(template_id_t, source_t)
F_t = fleet_impact(source_t, template_id_t, window)
P_t = inference_path_penalty(inference_path_t)

V_cloud(e_t) =
  w_r * R_t
  + w_u * U_t
  + w_n * N_t
  + w_d * D_t
  + w_f * F_t
  + w_p * P_t
```

字段含义：

- `R_t`：严重度，来自 edge-infer 的 `severity / 100`；
- `U_t`：校准后的不确定性，不直接相信模型原始 confidence；
- `N_t`：模板新颖度，首次出现或近期少见模板更高；
- `D_t`：历史边云分歧率，若某类模板过去经常被云端纠正，则更值得上云；
- `F_t`：影响范围，同一窗口内受影响设备越多，越值得上云；
- `P_t`：推理路径惩罚，`FALLBACK`、`EDGE_LLM`、低置信 `MODEL` 比 `RULE`、`CACHE` 更值得上云。

权重 `w` 不是手工固定常数，而是在历史验证集上通过约束优化得到，并在在线反馈中低频更新。离线初始化目标为：

```text
min_w sum_i Loss(route(e_i; w), y_i)
subject to w_j >= 0, sum_j w_j = 1
```

其中 `Loss` 同时惩罚高风险漏报、无收益上云、SLA 违约和隐私策略违反。论文中报告最终学习到的权重，并通过消融证明各特征的贡献。

### 4.2 不确定性校准与保序风险集合

边缘模型输出的 `confidence` 不能直接当作真实概率。RAISE 同时使用温度缩放和按路径分桶校准，将原始置信度转换成更可靠的不确定性：

```text
p_hat = Calibrator(edge_pred, confidence, severity, inference_path, template_freq)
U_t = 1 - p_hat
```

同时引入 conformal prediction 风险集合：

```text
C_t = {y | score_y(e_t) <= q_{1-alpha}}
U_t^{conf} = min(1, |C_t| / |Y|)
U_t = eta_u (1 - p_hat) + (1 - eta_u) U_t^{conf}
```

其中 `C_t` 越大，说明边缘预测可接受类别越多，路由器越倾向于上云复核。论文中报告 ECE、Brier Score、NLL 和 coverage，证明校准与 conformal 风险集合对路由质量的贡献。

### 4.3 云端收益预测与在线探索

并非所有上云都能带来收益。RAISE 维护一个轻量收益模型 `G(e_t)`，预测云端复核相比边缘结果的边际收益：

```text
G(e_t) = P(cloud_corrects_edge | features)
```

特征包括：

```text
severity, uncertainty, template_freq, source, edge_pred,
inference_path, historical_disagreement, recent_incident_context
```

最终版不使用经验分数，而是使用在线收益模型：

```text
G(e_t) = min(1, sigma(theta^T psi_t) + alpha_ucb sqrt(psi_t^T A^{-1} psi_t))
```

当云端结果返回后，比较 `edge_pred` 与 `cloud_pred/root_cause`，构造收益：

```text
reward =
  1.0, cloud corrects a wrong edge decision
  0.7, cloud adds actionable root cause or recommendation
  0.3, cloud confirms a high-risk edge decision
  0.0, cloud gives no additional information
```

该收益同时更新 `theta`、模板边云分歧率和新颖模板统计，使 RAISE 成为反馈驱动的自适应路由算法。

### 4.4 策略意图约束

工业系统不能完全交给学习算法。RAISE 将用户策略作为 hard constraint，而不是简单加权项：

```text
if matches(deny_cloud_rules, e_t):
  return LOCAL with reason=PRIVACY_DENY

if matches(force_cloud_rules, e_t) and cloud_available:
  return CLOUD with reason=FORCE_CLOUD_RULE

if matches(force_cloud_rules, e_t) and not cloud_available:
  return DEFER with reason=FORCE_CLOUD_NETWORK_DOWN

if matches(force_local_rules, e_t):
  return LOCAL with reason=FORCE_LOCAL_RULE
```

这部分称为 Intent-constrained routing。策略约束是 hard constraint，优先级高于收益模型和动态阈值，保证学习算法不会覆盖运维人员明确指定的合规、隐私、安全和生产策略。

### 4.5 在线预算与队列背压

RAISE 使用动态阈值和 token bucket 双层控制。动态阈值负责软调节上云倾向，token bucket 负责硬预算边界。动态阈值由云端预算、队列积压和网络健康共同决定：

```text
theta_t =
  theta_0
  + alpha_q * queue_pressure_t
  + alpha_b * budget_pressure_t
  + alpha_n * network_pressure_t
  - alpha_a * audit_pressure_t
```

其中：

```text
queue_pressure_t  = normalize(cloud_queue_lag + outbox_lag)
budget_pressure_t = 1 - budget_remaining_ratio
network_pressure_t = f(network_rtt_ms, network_fail_rate, cloud_health)
audit_pressure_t = max(0, min_cloud_ratio_for_audit - recent_cloud_ratio)
```

解释：

- 云端队列越堵，阈值越高，只有更有价值的事件上云；
- 预算越紧，阈值越高；
- 网络越差，立即上云阈值越高，但可转入 `DEFER`；
- 如果近期上云比例太低，为了持续校准和审计，阈值适当降低。

最终决策分数：

```text
S_t = beta_v * V_cloud(e_t) + beta_g * G(e_t) - beta_c * C_expected(e_t, s_t)
```

如果：

```text
S_t >= theta_t
```

则事件值得上云；否则本地处理。

### 4.6 弱网自治与最终一致性

当事件值得上云，但网络不可用或云端限流时：

```text
if worth_cloud(e_t) and not cloud_sendable(s_t):
  return DEFER
```

`DEFER` 不是失败，而是弱网自治策略。router 将事件写入 `cloud_outbox`：

```text
event_id
payload
priority_score
deadline_at
attempt
status=PENDING
defer_reason
created_at
next_retry_at
```

补传顺序按优先级：

```text
replay_priority =
  gamma_s * severity
  + gamma_u * uncertainty
  + gamma_age * waiting_time
  + gamma_rule * force_cloud_flag
```

这样可以保证网络恢复后，高风险和等待时间长的事件优先补传，避免 FIFO 把关键故障压在低价值日志后面。

## 5. RAISE 决策流程

```text
Input: inferred event e_t, system state s_t, policy p_t
Output: action in {LOCAL, CLOUD, DEFER}

1. Validate event schema and idempotency by event_id.
2. Apply hard policy constraints:
   - deny cloud -> LOCAL
   - force local -> LOCAL
   - force cloud -> CLOUD or DEFER
3. Compute calibrated uncertainty U_t.
4. Compute cloud value V_cloud(e_t).
5. Predict marginal cloud gain G(e_t).
6. Compute dynamic threshold theta_t from queue, budget and network pressure.
7. If S_t < theta_t:
      return LOCAL
8. If cloud is healthy, budget is available and rate limit allows:
      return CLOUD
9. Otherwise:
      write cloud_outbox and return DEFER
10. Emit routed event with reason, score and system snapshots.
```

伪代码：

```python
def route(event, state, policy):
    if is_duplicate(event.event_id):
        return cached_decision(event.event_id)

    hard = apply_hard_policy(event, policy, state)
    if hard is not None:
        persist_decision(event, hard)
        return hard

    u = calibrated_uncertainty(event)
    v = cloud_value(event, u)
    g = expected_cloud_gain(event, u)
    cost = expected_send_cost(event, state, policy)
    theta = dynamic_threshold(state, policy)

    score = BETA_V * v + BETA_G * g - BETA_C * cost

    if score < theta:
        decision = Decision("LOCAL", reason="LOW_CLOUD_VALUE", score=score)
    elif cloud_sendable(state, policy):
        decision = Decision("CLOUD", reason="HIGH_VALUE_SENDABLE", score=score)
        publish_cloud_request(event)
    else:
        decision = Decision("DEFER", reason=defer_reason(state), score=score)
        write_outbox(event, priority=outbox_priority(event, u))

    persist_decision(event, decision)
    publish_routed_event(event, decision, state)
    update_metrics(event, decision)
    return decision
```

## 6. 输出契约

`cotriage.events.routed` 增加算法解释字段：

```json
{
  "event_id": "01J...",
  "action": "CLOUD",
  "policy_ref": "raise-v1",
  "reason": "HIGH_UNCERTAINTY_HIGH_SEVERITY",
  "score": 0.83,
  "threshold": 0.61,
  "features": {
    "risk": 0.78,
    "uncertainty": 0.42,
    "novelty": 0.90,
    "historical_disagreement": 0.31,
    "fleet_impact": 0.20,
    "expected_gain": 0.48
  },
  "network_snapshot": {
    "rtt_ms": 86,
    "fail_rate": 0.02,
    "cloud_health": "UP"
  },
  "budget_snapshot": {
    "minute_remaining": 17,
    "day_remaining": 4312
  },
  "latency_ms": 4
}
```

`action=DEFER` 时需要额外记录：

```json
{
  "defer_reason": "NETWORK_DOWN",
  "outbox_id": 12345,
  "next_retry_at": "2026-06-30T12:00:30Z"
}
```

## 7. 理论性质

论文中可以主张并验证以下性质。

### 7.1 预算可行性

令 token bucket 控制每分钟与每日云调用预算。只要所有 `CLOUD` 动作都必须先成功消费 token，则：

```text
cloud_calls_per_window <= bucket_capacity + refill_rate * window
```

这给出云端调用预算的硬上界。

### 7.2 SLA 保护

动态阈值随队列积压上升而上升：

```text
theta_t increases with queue_pressure_t
```

当云端队列接近拥塞时，上云事件数量下降，使系统退化为更保守的边缘自治模式，从而保护实时链路。

### 7.3 弱网最终一致性

对满足 `worth_cloud(e_t)=true` 且未被隐私策略禁止的事件，如果网络在未来某一时间恢复，并且 outbox replayer 持续运行，则事件最终会被补传，除非超过最大重试并进入 DLQ。该性质可用 outbox 状态机证明。

```text
PENDING -> SENDING -> SENT -> DONE
                  \-> RETRY
                  \-> DLQ
```

### 7.4 可解释性

每个决策都保留 `score`、`threshold`、关键特征、策略命中、网络快照和预算快照，因此可以复现“为什么这条日志上云/不上云/暂缓”。

## 8. 论文创新点

本文贡献凝练为四点：

1. 提出面向工业日志 AIOps 的云边协同路由问题，将边缘不确定性、云端预算、网络退化、隐私策略和 SLA 统一建模为在线约束优化。
2. 设计 RAISE 路由算法，通过校准不确定性、云端收益预测和动态阈值实现风险感知的自适应上云决策。
3. 提出弱网自治的一致性补偿机制，将 `DEFER` 作为一等决策，并通过优先级 outbox 保证高风险事件在网络恢复后优先补传。
4. 在真实系统 CoTriage 中实现端到端原型，并在多设备日志流、弱网注入、预算收缩和故障爆发场景下评估准确率、成本、时延和可用性。

## 9. 实验设计

### 9.1 对比方法

实验同时比较外部代表方法和内部消融。只和 `All-Local`、`All-Cloud`、固定阈值这类朴素方法比较，容易被审稿人认为 baseline 偏弱。本文将对比方法分为四组。

第一组：朴素策略。

- `All-Local`：全部边缘处理；
- `All-Cloud`：全部上云；
- `Severity-Threshold`：严重度超过阈值上云；
- `Confidence-Threshold`：置信度低于阈值上云；
- `Static-Weighted`：固定加权分数上云；
- `Queue-Aware`：只考虑队列和预算的动态阈值；

第二组：改造已有云边协同或级联推理思想。

- `Early-Exit-style`：参考 early-exit/BranchyNet 思想，置信度高时边缘退出，置信度低时进入云端深模型。该 baseline 只使用置信度和早退阈值，不使用历史边云分歧、预算自适应和 outbox。
- `Cascade-style`：参考 NoScope 这类级联推理思想，按“规则/轻量模型/云端模型”构建固定级联，只有上一级拒判或低置信时才进入下一级。该 baseline 强调级联成本优化，但不考虑弱网和在线反馈。
- `Edgent-style`：参考边缘协同推理中的动态执行计划思想，根据网络带宽或 RTT 在本地和云端之间切换。该 baseline 主要由网络状态驱动，不引入日志风险、模板新颖度和边云分歧收益。
- `Bandit-Routing`：将上云视为一臂，留在本地视为另一臂，使用 LinUCB 或 Thompson Sampling 根据云端反馈学习动作收益。该 baseline 有在线学习，但不显式加入 operator hard constraints、动态 SLA 阈值和弱网 outbox 一致性。

第三组：约束优化与预算控制策略。

- `Budget-Greedy`：在预算允许时，按 `severity * uncertainty` 从高到低上云，预算耗尽后全部本地处理。
- `Lyapunov-style`：维护虚拟预算队列或积压队列，用队列长度惩罚上云动作，实现长期预算约束。该 baseline 适合证明 RAISE 不只是简单 token bucket。
- `SLA-Aware-Offloading`：按预测时延选择本地或云端，只优化时延和上云成本，不考虑诊断收益和校准不确定性。

第四组：RAISE 消融。

- `RAISE-no-calibration`：去掉不确定性校准；
- `RAISE-no-feedback`：去掉云端收益在线更新；
- `RAISE-no-outbox-priority`：弱网补传使用 FIFO。
- `RAISE-no-policy-constraint`：将用户策略当作软分数而非 hard constraint；
- `RAISE-no-dynamic-threshold`：使用固定阈值替代动态阈值；
- `RAISE-no-novelty-disagreement`：去掉模板新颖度和历史边云分歧特征。

论文主表放置以下代表方法：

```text
All-Local
All-Cloud
Confidence-Threshold
Early-Exit-style
Cascade-style
Edgent-style
Bandit-Routing
Budget-Greedy
RAISE
```

消融表单独放：

```text
RAISE-full
RAISE-no-calibration
RAISE-no-feedback
RAISE-no-outbox-priority
RAISE-no-dynamic-threshold
RAISE-no-policy-constraint
```

这样主表回答“比已有路线强在哪里”，消融表回答“RAISE 每个模块是否必要”。

### 9.2 指标

核心指标：

- 故障识别 F1、Macro-F1、Recall；
- 高严重度事件漏报率；
- 云调用比例；
- 平均 token 成本；
- P50/P95/P99 端到端时延；
- 弱网期间本地可用率；
- 网络恢复后的补传完成时间；
- 边云分歧事件捕获率；
- ECE 和 Brier Score；
- 决策解释覆盖率。

### 9.3 场景

实验构造四类场景：

1. 平稳日志流：验证成本下降与准确率保持；
2. 故障爆发：验证队列背压和高风险优先上云；
3. 弱网/断网：验证 `DEFER`、outbox 和最终一致性；
4. 模板漂移：验证新模板识别、在线收益更新和审计采样。

### 9.4 预期结果叙事

目标不是证明 RAISE 准确率一定超过 All-Cloud，而是证明：

- 接近 All-Cloud 的故障识别效果；
- 显著低于 All-Cloud 的云调用成本；
- 明显优于 All-Local 的高风险事件召回；
- 在弱网下比所有立即上云策略更稳定；
- 在预算收缩时比固定阈值策略更平滑退化。

这类“多目标 trade-off”更符合 TPDS/TSC/TC 对系统算法论文的期待。

## 10. 最终实现要求

本节给出 RAISE 最终论文版在 CoTriage 中的实现要求。系统实现应完整包含校准、在线收益学习、动态阈值、策略约束、预算控制和优先级 outbox。

### 10.1 必须实现的算法状态

router-controller 需要维护以下状态：

- `calibrator_state`：按 `edge_pred + inference_path` 分桶的校准参数；
- `conformal_state`：各类别非一致性分数分布和分位数；
- `template_stats`：模板频率、新颖度、历史上云次数和边云分歧率；
- `gain_model_state`：在线收益模型参数 `theta`、`A`、`b`；
- `impact_window`：滑动窗口内模板影响设备数和异常增长率；
- `budget_state`：分钟级、小时级、日级 token bucket；
- `network_state`：RTT、失败率、云端健康状态；
- `queue_state`：cloud queue、outbox、Redis consumer lag；
- `policy_state`：当前生效策略版本、强制上云/本地/禁止上云规则；
- `outbox_state`：待补传事件、优先级、重试次数和 deadline。

### 10.2 必须落库的可复现实验字段

为了支撑论文实验和审稿复现，每条事件至少记录：

- `event_id`、`template_id`、`source`、`event_ts`；
- 边缘输出：`edge_pred`、`severity`、`confidence`、`inference_path`、`model_version`；
- 校准输出：`p_hat`、`uncertainty`、`conformal_set_size`；
- 价值特征：`novelty`、`disagreement_rate`、`fleet_impact`、`path_risk`；
- 收益估计：`expected_gain`、`ucb_bonus`、`gain_model_version`；
- 成本估计：`cloud_cost`、`latency_cost`、`network_cost`、`privacy_cost`；
- 系统状态：`queue_pressure`、`budget_pressure`、`network_pressure`、`audit_pressure`；
- 决策输出：`score`、`threshold`、`action`、`reason`、`policy_ref`；
- 云端反馈：`cloud_pred`、`root_cause`、`recommendation`、`cloud_latency_ms`、`reward`；
- 弱网字段：`defer_reason`、`outbox_priority`、`attempt`、`replayed_at`。

### 10.3 参数训练与更新流程

最终版 RAISE 采用离线初始化和在线更新结合：

1. 使用历史标注日志训练边缘模型并生成校准集；
2. 在校准集上学习温度参数、分桶校准参数和 conformal 分位数；
3. 使用历史边云对照结果初始化 `theta`、`A`、`b` 和价值函数权重；
4. 在线运行时根据云端返回结果更新收益模型和模板分歧率；
5. 周期性将在线状态快照落库，并在 router 重启后恢复；
6. 每个策略版本、模型版本和校准版本均进入决策日志。

### 10.4 不采用深度强化学习的理由

最终论文设计不采用深度强化学习作为主算法。原因是工业日志路由动作空间小、策略约束硬、反馈延迟且带噪，深度 RL 会显著增加训练稳定性、可解释性和安全性审稿风险。RAISE 使用在线学习、上置信界探索和受约束优化，能够在保持可解释和可证明性质的同时获得自适应能力。

## 11. 与现有 CoTriage 架构的对应关系

```text
edge-infer output
  -> RAISE feature extractor
  -> policy constraint checker
  -> uncertainty calibrator
  -> value and gain estimator
  -> dynamic threshold controller
  -> action dispatcher
       LOCAL -> cotriage.events.routed -> server落库
       CLOUD -> cotriage.events.cloud.req
       DEFER -> cloud_outbox -> outbox-replayer -> cotriage.events.cloud.req
```

router-controller 需要维护的运行时状态：

- template 频率表；
- template 边云分歧率；
- source 影响范围窗口；
- token bucket；
- network health probe；
- queue lag watcher；
- outbox priority queue；
- calibration 参数；
- online gain model 参数。

这些状态应在内存中提供低延迟访问，并周期性落库以支持重启恢复和论文复现实验。多副本部署时，router-controller 需要通过 leader election 或一致性分片保证同一 `template_id` 的统计更新不会被并发写乱。
