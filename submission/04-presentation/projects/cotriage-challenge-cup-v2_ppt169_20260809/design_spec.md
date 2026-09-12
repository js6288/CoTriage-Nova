# CoTriage Challenge Cup Defense V2 - Design Spec

> Human-readable design narrative for the 10-minute function-first defense deck.
>
> Machine-readable execution contract: `spec_lock.md`. The Executor re-reads that file before every page; on divergence, `spec_lock.md` wins.

## I. Project Information

| Item | Value |
| ---- | ----- |
| **Project Name** | CoTriage Challenge Cup Defense V2 |
| **Canvas Format** | PPT 16:9 (1280×720) |
| **Page Count** | 19 pages: 15 main-defense pages + 4 Q&A appendix pages |
| **Design Style** | B) General Consulting + enterprise-grade product demonstration |
| **Target Audience** | Challenge Cup judges, enterprise technical reviewers, and sponsoring-unit experts |
| **Use Case** | 10-minute competition defense with a short system demonstration and Q&A backup |
| **Core Message** | CoTriage turns multi-node logs into unified events, performs layered edge inference, routes each event through LOCAL/CLOUD/DEFER, and exposes the complete operating state through a usable management system. |
| **Created Date** | 2026-08-09 |

---

## II. Canvas Specification

| Property | Value |
| -------- | ----- |
| **Format** | PPT 16:9 |
| **Dimensions** | 1280×720 |
| **viewBox** | `0 0 1280 720` |
| **Margins** | 48px left/right, 34px top, 28px bottom |
| **Content Area** | 1184×610px below the title band |
| **Main-title band** | y=30–88, conclusion-first title aligned left |
| **Footer band** | y=686–708, page number and short section label only |

---

## III. Visual Theme

### Theme Style

- **Style**: B) General Consulting + enterprise-grade product demonstration
- **Theme**: Light theme
- **Tone**: credible, operational, technically disciplined, concise, and competition-ready
- **Visual principle**: real system screens are the hero assets; native SVG carries conclusions, labels, flows, and boundaries.
- **Public-material principle**: no evidence IDs, source-state labels, internal paths, contest logos, fabricated numbers, or unfinished-result placeholders.

### Color Scheme

| Role | HEX | Purpose |
| ---- | --- | ------- |
| **Background** | `#F6F8FB` | Main page background |
| **Surface** | `#FFFFFF` | Screenshot frame and compact content panels |
| **Primary** | `#163A5F` | Titles, architecture modules, strong anchors |
| **Accent** | `#16A6B6` | Data flow, active links, operating-state emphasis |
| **Decision accent** | `#F59E0B` | RAISE decisions, route thresholds, key transitions |
| **Danger** | `#C62828` | Fault or failure only |
| **Success** | `#2E7D32` | Ready/healthy state only |
| **Body text** | `#1D2F42` | Primary text |
| **Secondary text** | `#60758A` | Captions and qualifiers |
| **Border/divider** | `#CBD7E3` | Frames and separators |
| **Muted fill** | `#E9EEF3` | Quiet regions and secondary chips |

No AI-generated image rows are used. All visual evidence is user-supplied and already present in the project.

### Gradient Scheme

Gradients are limited to subtle native SVG scrims over screenshots. Use `stop-opacity`, never `rgba()`.

---

## IV. Typography System

### Font Plan

**Typography direction**: CJK-first enterprise sans, with weight and scale contrast rather than a second decorative family.

| Role | Chinese | English | Fallback tail |
| ---- | ------- | ------- | ------------- |
| **Title** | `Microsoft YaHei` | `Arial` | `sans-serif` |
| **Body** | `Microsoft YaHei` | `Arial` | `sans-serif` |
| **Emphasis** | `Microsoft YaHei` | `Arial` | `sans-serif` |
| **Code** | — | `Consolas`, `Courier New` | `monospace` |

**Per-role font stacks**:

- Title: `"Microsoft YaHei", Arial, sans-serif`
- Body: `"Microsoft YaHei", Arial, sans-serif`
- Emphasis: same as Body; use weight and color contrast
- Code: `Consolas, "Courier New", monospace`

### Font Size Hierarchy

**Baseline**: Body font size = 20px.

| Purpose | Size | Weight |
| ------- | ---- | ------ |
| Cover title | 64px | 700 |
| Page title | 38px | 700 |
| Subtitle / takeaway | 26px | 600 |
| Body content | 20px | 400–600 |
| Screenshot label | 16px | 600 |
| Annotation / caption | 14px | 400 |
| Page number / footnote | 12px | 400 |

---

## V. Layout Principles

### Page Structure

- **Header area**: y=30–90; conclusion-style title on the left, optional short section label on the right.
- **Content area**: y=104–670; screenshots or one primary structural visualization dominate.
- **Footer area**: y=686–708; page number and section only; no evidence IDs or repeating project slogan.
- **Screenshot policy**: screenshots are never stretched; dense UI images use `meet` and retain every original pixel.
- **Callout policy**: at most three callouts on a screenshot page; no callout may cover a displayed metric or diagnosis field.

### Layout Pattern Library

| Pattern | Usage in this deck |
| ------- | ------------------ |
| **Single column centered** | Cover and final close |
| **Vertical pillars** | Three unsuitable legacy approaches on P02 |
| **Image-as-canvas with native overlay** | Product reveal, realtime dashboard, RAISE status |
| **Top-bottom split** | Ultra-wide event stream and diagnosis detail |
| **Asymmetric collage** | Device management plus realtime event stream |
| **Diptych / picture-in-picture** | Edge inference and cloud diagnosis |
| **Filmstrip** | One complete demonstration path |
| **Layered architecture** | Module deployment and two-scenario adaptation |
| **Compact table** | Appendix metric definitions only |

### Spacing Specification

**Universal**:

| Element | Recommended Range | Current Project |
| ------- | ---------------- | --------------- |
| Safe margin from canvas edge | 40–60px | 48px |
| Content block gap | 24–40px | 28px |
| Icon-text gap | 8–16px | 12px |

**Card-based layouts**:

| Element | Recommended Range | Current Project |
| ------- | ---------------- | --------------- |
| Card gap | 20–32px | 24px |
| Card padding | 20–32px | 24px |
| Card border radius | 8–16px | 12px |
| Three-column card width | 360–380px | 370px |

**Non-card containers**:

- Line-height: 1.45× body size.
- Screenshot captions sit outside image pixels.
- Breathing pages use whitespace, one dominant image, and one sentence of conclusion.

---

## VI. Icon Usage Specification

### Source

- **Built-in icon library**: `chunk-filled` only.
- **Usage method**: SVG placeholder `<use data-icon="chunk-filled/icon-name" .../>`.
- **Brand logos**: disabled.

### Recommended Icon List

| Purpose | Icon Path | Page |
| ------- | --------- | ---- |
| Industrial scene | `chunk-filled/factory` | P02, P14 |
| Edge/server node | `chunk-filled/server` | P05, P06, P14 |
| Storage | `chunk-filled/database` | P05, P09, P19 |
| Cloud capability | `chunk-filled/cloud` | P02, P05, P10 |
| Security and governance | `chunk-filled/shield-check` | P11, P15 |
| Runtime metrics | `chunk-filled/chart-line` | P04, P13, P16 |
| Online configuration | `chunk-filled/sliders` | P11, P19 |
| Weak network | `chunk-filled/wifi-slash` | P09, P18 |
| Routing | `chunk-filled/route` | P08, P12, P17 |
| Layered inference | `chunk-filled/layers` | P07, P15 |
| Event record | `chunk-filled/file` | P06, P12 |
| Retry / time | `chunk-filled/clock` | P09, P18 |
| Fault marker | `chunk-filled/circle-exclamation` | P02, P07 |
| Model inference | `chunk-filled/robot` | P07, P10 |
| Innovation | `chunk-filled/lightbulb` | P15 |
| Completion | `chunk-filled/checkmark` | P03, P12 |
| End-to-end link | `chunk-filled/link` | P05, P12 |
| Observability | `chunk-filled/eye` | P04, P13 |
| Privacy | `chunk-filled/lock-closed` | P10, P19 |

---

## VII. Visualization Reference List

Catalog read: 71 templates.

| Page | Template | Path | Summary-quote (verbatim from `charts_index.json`) | Usage |
| ---- | -------- | ---- | ------------------------------------------------- | ----- |
| P02 | vertical_pillars | `templates/charts/vertical_pillars.svg` | "Pick for 1×3 / 1×4 / 1×5 vertical column layout where each pillar = one independent category with title + bullets — PEST (Political/Economic/Social/Technological), four-pillar strategy overview, side-by-side independent categories. Skip for 2×2 quadrant (use quadrant_text_bullets), pricing tiers (use comparison_columns), or 2×2 parallel aspects (use labeled_card)." | Three conventional approaches and their limits |
| P09 | process_flow | `templates/charts/process_flow.svg` | "Pick for 3-8 sequential steps connected by simple arrows — approval workflows, customer onboarding, request handling, lifecycle stages. Skip if cyclical (use circular_stages) or stages produce named outputs (use pipeline_with_stages)." | DEFER and Outbox state flow |
| P12 | pipeline_with_stages | `templates/charts/pipeline_with_stages.svg` | "Pick for 3-5 horizontal pipeline stages, each = title + 1-line description + output artifact, connected by arrows (data pipelines, ETL, build pipelines). Skip if any stage lacks an artifact (use process_flow or numbered_steps)." | Demonstration path and visible artifact at each stage |
| P14 | layered_architecture | `templates/charts/layered_architecture.svg` | "Pick for 3-4 horizontal architecture layers (presentation/service/data), 2-4 module cards per layer, each card = title + 1-line description (description required, even if source brief). Skip if no per-module descriptions (use icon_grid) or no horizontal layering (use module_composition)." | Deployment, standard interfaces, and scenario adaptation |
| P15 | icon_grid | `templates/charts/icon_grid.svg` | "Pick for 4-9 parallel features/capabilities/services as icon cards — feature grid, service lineup, benefits matrix, brand values, product highlights. Skip for sequential ordering (use numbered_steps) or hierarchical layers (use pyramid_chart)." | Four collaborative innovations |
| P16 | basic_table | `templates/charts/basic_table.svg` | "Pick for plain tabular text/number grid, 3-8 columns. Skip if cells need visual bars (use consulting_table) or qualitative scores (use harvey_balls_table)." | Metric definition and interpretation table |
| P17 | hub_inward_arrows | `templates/charts/hub_inward_arrows.svg` | "Pick for 1 center subject + 4-5 surrounding forces with arrows pointing inward — Porter's Five Forces (central rivalry, threat of new entrants/substitutes, bargaining power of buyers/suppliers), threat-model with central asset, pressure-on-core diagrams. Skip for outward radiation from center capability (use hub_spoke) or non-directional cluster (use mind_map)." | Risk, uncertainty, network, budget, and policy inputs to RAISE |
| P18 | process_flow | `templates/charts/process_flow.svg` | "Pick for 3-8 sequential steps connected by simple arrows — approval workflows, customer onboarding, request handling, lifecycle stages. Skip if cyclical (use circular_stages) or stages produce named outputs (use pipeline_with_stages)." | Outbox retry and dead-letter state machine |

**Runners-up considered**:

- `numbered_steps` | rejected for P12 because every demonstration stage has a visible output artifact, so `pipeline_with_stages` is more specific.
- `hub_spoke` | rejected for P17 because the factors constrain and feed the central route decision rather than radiate outward.
- `kpi_cards` | rejected for P13 because the source dashboard must be preserved as a complete real screenshot instead of redrawing its values.

---

## VIII. Image Resource List

| Filename | Dimensions | Ratio | Purpose | Type | Layout pattern | Acquire Via | Status | Reference | text_policy | page_role |
| -------- | ---------- | ----- | ------- | ---- | -------------- | ----------- | ------ | --------- | ----------- | --------- |
| `cotriage-system-architecture.png` | 3840×2160 | 1.78 | Enterprise system architecture | Diagram | #19 Image floating in whitespace with thin frame and caption | user | Existing | Complete cloud-edge architecture with readable module labels | | |
| `s-00-application-homepage.png` | 2940×1654 | 1.78 | Product reveal | Diagram | #1 Full-bleed background with floating title + #30 Flat semi-transparent rectangle overlay | user | Existing | CoTriage application home page | | |
| `s-01-realtime-dashboard.png` | 2940×1832 | 1.60 | Runtime cockpit | Diagram | #45 Background image + numbered hotspots with sidebar legend + #21 Rounded rectangle crop | user | Existing | Realtime ingestion, inference, anomaly, routing, and Outbox overview | | |
| `s-02-device-management.png` | 2940×1836 | 1.60 | Multi-node registration | Diagram | #49 Asymmetric collage + #21 Rounded rectangle crop | user | Existing | Four ready device types | | |
| `s-02c-inference-trend-and-distribution.png` | 2936×1662 | 1.77 | Layered inference distribution | Diagram | #55 Diptych split — two images abutting at 50/50 + #21 Rounded rectangle crop | user | Existing | RULE/CACHE/MODEL path distribution and event trend | | |
| `s-02d-realtime-event-stream-cache-model.png` | 2892×1200 | 2.41 | Unified event stream | Diagram | #49 Asymmetric collage + #21 Rounded rectangle crop | user | Existing | CACHE and MODEL events in one live stream | | |
| `s-03-anomaly-trend-analysis-redacted.png` | 1906×825 | 2.31 | Anomaly situation | Diagram | #55 Diptych split — two images abutting at 50/50 + #21 Rounded rectangle crop | user | Existing | Redacted historical-log replay with risk and priority | | |
| `s-03b-anomaly-diagnosis-center-overview.png` | 2880×1516 | 1.90 | Diagnosis center overview | Diagram | #17 Picture-in-picture inset + #21 Rounded rectangle crop | user | Existing | Anomaly trend, risk aggregation, cloud root cause, and event stream | | |
| `s-04a-raise-routing-status-overview.png` | 2902×1382 | 2.10 | RAISE operating state | Diagram | #46 Background image + bordered "lens" rectangle highlighting a sub-region + #21 Rounded rectangle crop | user | Existing | Route pressure, threshold, budget, cloud health, and Outbox status | | |
| `s-05a-cloud-root-cause-diagnosis.png` | 2852×760 | 3.75 | Structured cloud diagnosis detail | Diagram | #17 Picture-in-picture inset + #21 Rounded rectangle crop | user | Existing | Root cause, recommendation, explanation, latency, confidence, and Token display | | |
| `s-08a-rule-management.png` | 2940×1836 | 1.60 | Rule governance | Diagram | #56 Image triptych + #21 Rounded rectangle crop | user | Existing | Online rule management | | |
| `s-09a-minilm-model-management.png` | 2940×1836 | 1.60 | Edge model governance | Diagram | #56 Image triptych + #21 Rounded rectangle crop | user | Existing | Published MiniLM configuration | | |
| `s-09c-cloud-provider-management.png` | 2940×1834 | 1.60 | Cloud Provider governance | Diagram | #56 Image triptych + #21 Rounded rectangle crop | user | Existing | Masked cloud Provider configuration | | |
| `s-11-competition-metrics-dashboard.png` | 2936×1334 | 2.20 | Current runtime-window metric dashboard | Diagram | #19 Image floating in whitespace with thin frame and caption | user | Existing | Complete dashboard retained without redrawing or cropping its comparative field | | |

Image-as-canvas coverage is provided by P04 (`#45`) and P08 (`#46`). All screenshots are no-crop assets in `spec_lock.md`.

---

## IX. Content Outline

### Part 1 — Problem and Product Reveal

#### Slide 01 — CoTriage: high-frequency logs handled nearby, high-value problems diagnosed on demand

- **Layout**: negative-space cover with a native edge-to-cloud mechanism line.
- **Title**: CoTriage：让高频日志就近分诊，让高价值问题按需获得云端能力
- **Subtitle**: 面向工业多设备日志的云边协同智能故障感知与决策系统
- **Info**: XH-202606 挑战杯作品答辩｜2026.08

#### Slide 02 — All-edge lacks context and all-cloud sacrifices immediacy

- **Layout**: three vertical pillars plus a bottom answer strip.
- **Title**: 全边缘看不全、全上云不够快，工业现场需要事件级云边协同
- **Visualization**: vertical_pillars.
- **Content**: rule-only coverage limit; all-edge context limit; all-cloud network/cost/privacy limit; CoTriage chooses a path per event.

#### Slide 03 — A running and operable cloud-edge diagnosis product already exists

- **Layout**: full-bleed S-00 with a quiet lower title strip.
- **Title**: 我们已经构建了一套可运行、可操作的云边协同日志诊断系统
- **Content**: product entry, realtime cockpit, anomaly center, configuration center.

#### Slide 04 — One cockpit exposes ingestion, inference, routing, anomaly and reliable-transfer state

- **Layout**: S-01 as image canvas with five numbered hotspots and a compact side legend.
- **Title**: 多节点日志、推理状态和路由结果已在统一驾驶舱中实时呈现
- **Content**: realtime flow; inference throughput; anomaly situation; RAISE route state; Outbox visibility.

### Part 2 — Architecture and Six Core Functions

#### Slide 05 — Six core components form a traceable event-driven loop

- **Layout**: architecture image centered with a narrow takeaway rail.
- **Title**: 六个核心组件围绕统一事件形成采集—推理—路由—诊断闭环
- **Content**: collector, stream, edge inference, RAISE, cloud gateway, feedback/management; shared event identity.

#### Slide 06 — Function 1: multi-node logs become one traceable event stream

- **Layout**: asymmetric collage; device list on the left, ultra-wide event stream on the right/bottom.
- **Title**: 功能一：多节点日志就近采集，并转化为可追踪的统一事件流
- **Content**: four ready node types; unified event fields; continuous CACHE/MODEL events; one identity across the chain.

#### Slide 07 — Function 2: inference escalates from deterministic paths to compact models

- **Layout**: dominant S-02c with S-03 redacted inset and four-step native ladder.
- **Title**: 功能二：规则、模板、轻量模型和小语言模型按成本分层推理
- **Content**: RULE/CACHE fast path; MiniLM; edge LLM; explicit degradation; historical-log replay note.

#### Slide 08 — Function 3: RAISE makes state-aware LOCAL/CLOUD/DEFER decisions

- **Layout**: S-04a-2 as dominant image with three action labels and two lens outlines.
- **Title**: 功能三：RAISE 联合风险、不确定性、网络和预算选择 LOCAL/CLOUD/DEFER
- **Content**: joint inputs; dynamic threshold; action semantics; route state remains observable.

#### Slide 09 — Function 4: DEFER and Outbox keep cloud requests off the realtime critical path

- **Layout**: native five-step process flow with one orange branch to retry/dead-letter.
- **Title**: 功能四：DEFER 与 Outbox 让弱网事件不阻塞现场实时主链
- **Visualization**: process_flow.
- **Content**: local result first; durable enqueue; priority read; due retry; sent or dead-letter trail. Do not claim measured recovery success.

#### Slide 10 — Function 5: cloud context becomes structured root cause and action advice

- **Layout**: S-03b large overview plus S-05a ultra-wide detail inset.
- **Title**: 功能五：云端聚合关联上下文，输出根因、建议和结构化解释
- **Content**: selected events only; context aggregation; redaction; root cause, recommendation, explanation, confidence, latency and Token fields.

#### Slide 11 — Function 6: rules, edge models and cloud providers are governed online

- **Layout**: three-image triptych with one governance rail.
- **Title**: 功能六：规则、边缘模型和云端 Provider 均可在线治理
- **Content**: rule priorities; published MiniLM; masked Provider credentials; versioned configuration without rewriting the main chain.

### Part 3 — Demonstration, Runtime Window and Value

#### Slide 12 — One demonstration follows the same five-stage event journey

- **Layout**: four screenshot filmstrip above a five-stage pipeline.
- **Title**: 一次完整演示沿“接入—识别—路由—深诊—回写”展开
- **Visualization**: pipeline_with_stages.
- **Content**: S-01 → S-02d → S-04a-2 → S-05a; each stage names the visible artifact.

#### Slide 13 — The system already exposes one unified competition-metric observation surface

- **Layout**: complete S-11 centered, no crop, with one top label “当前运行窗口” and one bottom interpretation line.
- **Title**: 系统已经围绕赛事要求建立统一的运行指标观测面
- **Content**: realtime, effect/consistency, resource/communication, stability/extension categories. Do not extract or reinterpret displayed numbers.

#### Slide 14 — Standard interfaces support two scenario families and replaceable intelligence

- **Layout**: three-layer architecture with two scenario ribbons.
- **Title**: 模块化部署和统一事件接口支持工业服务与分布式基础设施两类场景接入
- **Visualization**: layered_architecture.
- **Content**: presentation layer; service layer; Redis/MySQL layer; industrial-service scenario; distributed-infrastructure scenario; replaceable model/Provider.

#### Slide 15 — Innovation is the collaboration mechanism, not a single model

- **Layout**: four-icon capability grid with one closing sentence.
- **Title**: CoTriage 已将多节点感知、实时分诊和按需深诊组织成可演示闭环
- **Visualization**: icon_grid.
- **Content**: layered inference; state-aware routing; weak-network defer; online governance; application value.

### Part 4 — Q&A Appendix

#### Slide 16 — Appendix A: metric definitions and interpretation boundaries

- **Layout**: compact four-column table.
- **Title**: 附录 A｜赛事指标定义与统计口径
- **Visualization**: basic_table.
- **Content**: average vs P95; TTFT vs full response; business retention; conflict rate; conflict-resolution rate; zero-denominator handling.

#### Slide 17 — Appendix B: how one RAISE decision is formed

- **Layout**: center RAISE node with five inward factors and three output actions.
- **Title**: 附录 B｜RAISE 单事件决策解释
- **Visualization**: hub_inward_arrows.
- **Content**: risk, uncertainty, network, budget, policy; dynamic threshold; LOCAL/CLOUD/DEFER.

#### Slide 18 — Appendix C: Outbox retry and dead-letter state machine

- **Layout**: six-node native process with status colors and boundary note.
- **Title**: 附录 C｜Outbox 重试与死信状态机
- **Visualization**: process_flow.
- **Content**: PENDING/RETRY → SENDING → SENT; failed attempt returns to retry or dead-letter; no measured recovery-rate claim.

#### Slide 19 — Appendix D: deployment and complete configuration governance

- **Layout**: top deployment rail plus three configuration thumbnails.
- **Title**: 附录 D｜工程部署与完整配置管理
- **Content**: collector/infer/router/gateway/server; Redis Streams; MySQL; REST/SSE; rule/model/Provider configuration.

---

## X. Speaker Notes Requirements

- One note file per page, filename matching the SVG basename.
- `notes/total.md` uses one `#` heading per slide; split files contain no headings.
- Main pages total 8:55, leaving at least 1:05 for switching and pauses.
- Tone: formal but conversational; conclusion first, then point to the visible UI.
- Each main note contains: opening sentence, two or three observable facts, one boundary sentence when required, and a transition.
- Appendix notes answer likely judge questions and are not counted in the main duration.
- No evidence IDs, source-status wording, experimental placeholders, or unverified “达标/领先” claims.

---

## XI. Technical Constraints Reminder

### SVG Generation Must Follow

1. `viewBox="0 0 1280 720"`.
2. Background uses native `<rect>` elements.
3. Text wrapping uses `<tspan>`; `<foreignObject>` is forbidden.
4. Transparency uses `fill-opacity` / `stroke-opacity`; `rgba()` is forbidden.
5. Forbidden: `<mask>`, `<style>`, `class`, `textPath`, `animate*`, `script`, `iframe`, `symbol` + `use`.
6. `<g opacity>` is forbidden; opacity belongs on individual children.
7. XML reserved characters are escaped; HTML named entities are forbidden.
8. `clipPath` is allowed only on `<image>` elements and contains one native shape.
9. Screenshot images use `preserveAspectRatio="xMidYMid meet"` and must not crop data.
10. Top-level object groups require stable `id` values for PowerPoint conversion.

### PPT Compatibility Rules

- All fonts are PPT-safe.
- No private-IP screenshot or unmasked credential may enter the project.
- S-11 stays whole; no metric value is redrawn.
- Red is reserved for actual fault/failure semantics.
- Every slide has one primary conclusion and one dominant visual structure.
