## canvas
- viewBox: 0 0 1280 720
- format: PPT 16:9

## colors
- bg: #F6F8FB
- surface: #FFFFFF
- primary: #163A5F
- accent: #16A6B6
- decision: #F59E0B
- danger: #C62828
- success: #2E7D32
- text: #1D2F42
- text_secondary: #60758A
- border: #CBD7E3
- muted: #E9EEF3

## typography
- font_family: Microsoft YaHei, Arial, sans-serif
- code_family: Consolas, Courier New, monospace
- body: 20
- title: 38
- subtitle: 26
- annotation: 14
- cover_title: 64
- footnote: 12

## icons
- library: chunk-filled
- inventory: factory, server, database, cloud, shield-check, chart-line, sliders, wifi-slash, route, layers, file, clock, circle-exclamation, robot, lightbulb, checkmark, link, eye, lock-closed

## images
- architecture: images/cotriage-system-architecture.png | no-crop
- application_home: images/s-00-application-homepage.png | no-crop
- realtime_dashboard: images/s-01-realtime-dashboard.png | no-crop
- devices: images/s-02-device-management.png | no-crop
- inference_distribution: images/s-02c-inference-trend-and-distribution.png | no-crop
- event_stream: images/s-02d-realtime-event-stream-cache-model.png | no-crop
- anomaly_redacted: images/s-03-anomaly-trend-analysis-redacted.png | no-crop
- diagnosis_overview: images/s-03b-anomaly-diagnosis-center-overview.png | no-crop
- raise_status: images/s-04a-raise-routing-status-overview.png | no-crop
- cloud_diagnosis: images/s-05a-cloud-root-cause-diagnosis.png | no-crop
- rules: images/s-08a-rule-management.png | no-crop
- minilm: images/s-09a-minilm-model-management.png | no-crop
- cloud_provider: images/s-09c-cloud-provider-management.png | no-crop
- competition_metrics: images/s-11-competition-metrics-dashboard.png | no-crop

## page_rhythm
- P01: anchor
- P02: dense
- P03: breathing
- P04: dense
- P05: dense
- P06: dense
- P07: dense
- P08: anchor
- P09: anchor
- P10: dense
- P11: dense
- P12: dense
- P13: breathing
- P14: dense
- P15: breathing
- P16: dense
- P17: dense
- P18: dense
- P19: dense

## page_charts
- P02: vertical_pillars
- P09: process_flow
- P12: pipeline_with_stages
- P14: layered_architecture
- P15: icon_grid
- P16: basic_table
- P17: hub_inward_arrows
- P18: process_flow

## forbidden
- Mixing icon libraries
- rgba()
- `<style>`, `class`, `<foreignObject>`, `textPath`, `@font-face`, `<animate*>`, `<script>`, `<iframe>`, `<symbol>`+`<use>`
- `<g opacity>`
- HTML named entities in text
- Private-IP screenshots
- Fabricated experiment values
- Evidence IDs or evidence-status labels in public slides
- Baseline matrices, ablation results, or empty experiment placeholders
