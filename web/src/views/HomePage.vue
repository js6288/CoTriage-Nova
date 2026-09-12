<template>
  <main class="landing">
    <div class="backdrop" aria-hidden="true"></div>
    <div class="ambient" aria-hidden="true"></div>

    <nav class="nav">
      <button class="brand" aria-label="CoTriage 首页">
        <span class="brand-mark"><span></span></span>
        <span class="brand-copy"><b>CoTriage</b><small>EDGE–CLOUD INTELLIGENCE</small></span>
      </button>
      <div class="nav-actions">
        <span class="system-badge"><i></i> 云边协同系统</span>
        <a-button class="nav-button" ghost @click="enterManage">管理中心</a-button>
      </div>
    </nav>

    <section class="hero">
      <div class="hero-copy">
        <div class="eyebrow"><span>INTELLIGENT OPERATIONS</span><i></i><span>REAL-TIME</span></div>
        <h1>让每一条日志<br /><em>在云边之间被理解</em></h1>
        <p class="lead">面向分布式基础设施的智能运维平台。边缘侧实时感知与快速决策，关键异常按需上云，由大模型完成结构化根因诊断。</p>

        <div class="actions">
          <a-button type="primary" size="large" class="primary-action" @click="enterDashboard">
            进入实时驾驶舱 <ArrowRightOutlined />
          </a-button>
          <a-button size="large" class="secondary-action" @click="enterManage">
            打开管理中心
          </a-button>
        </div>

        <div class="trust-row">
          <span><CheckCircleFilled /> 边缘实时推理</span>
          <span><CheckCircleFilled /> 动态路由决策</span>
          <span><CheckCircleFilled /> 弱网可靠补偿</span>
        </div>
      </div>

      <div class="visual-note" aria-hidden="true">
        <span class="note-line"></span>
        <div><small>EDGE → CLOUD</small><b>协同智能中枢</b></div>
      </div>
    </section>

    <section class="capabilities" aria-label="平台核心能力">
      <div class="capability">
        <span class="capability-icon cyan"><ThunderboltOutlined /></span>
        <div><b>毫秒级响应</b><small>边缘侧就近分析与处置</small></div>
      </div>
      <div class="capability">
        <span class="capability-icon blue"><ApartmentOutlined /></span>
        <div><b>智能分流</b><small>LOCAL · CLOUD · DEFER</small></div>
      </div>
      <div class="capability">
        <span class="capability-icon violet"><CloudServerOutlined /></span>
        <div><b>深度诊断</b><small>云端大模型结构化分析</small></div>
      </div>
      <div class="capability">
        <span class="capability-icon amber"><SafetyCertificateOutlined /></span>
        <div><b>可靠运行</b><small>弱网补传与全链路追踪</small></div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import {
  ApartmentOutlined,
  ArrowRightOutlined,
  CheckCircleFilled,
  CloudServerOutlined,
  SafetyCertificateOutlined,
  ThunderboltOutlined
} from '@ant-design/icons-vue';

const router = useRouter();

function enterManage() {
  router.push('/manage');
}

function enterDashboard() {
  router.push('/dashboard');
}
</script>

<style scoped>
.landing {
  position: relative;
  min-height: 100vh;
  overflow: hidden;
  color: #f7fbff;
  background: #020c20;
  isolation: isolate;
}
.backdrop {
  position: absolute;
  z-index: -3;
  inset: 0;
  background-image: url('/photo/cotriage-page.png');
  background-position: center right;
  background-size: cover;
  transform: scale(1.015);
}
.ambient {
  position: absolute;
  z-index: -2;
  inset: 0;
  background:
    linear-gradient(90deg, rgba(2, 10, 27, .96) 0%, rgba(2, 12, 31, .88) 27%, rgba(3, 18, 48, .28) 58%, rgba(2, 10, 26, .05) 100%),
    linear-gradient(180deg, rgba(0, 7, 20, .5) 0%, transparent 30%, transparent 68%, rgba(0, 7, 19, .85) 100%);
}
.landing::before {
  content: '';
  position: absolute;
  z-index: -1;
  inset: 0;
  pointer-events: none;
  background-image: linear-gradient(rgba(75, 137, 206, .025) 1px, transparent 1px), linear-gradient(90deg, rgba(75, 137, 206, .025) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: linear-gradient(90deg, #000, transparent 67%);
}
.nav {
  position: relative;
  z-index: 2;
  display: flex;
  width: min(1440px, calc(100% - 96px));
  height: 92px;
  margin: 0 auto;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(134, 190, 255, .13);
}
.brand {
  display: flex;
  align-items: center;
  gap: 13px;
  padding: 0;
  color: inherit;
  border: 0;
  background: transparent;
  cursor: default;
}
.brand-mark {
  position: relative;
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid rgba(65, 190, 255, .7);
  border-radius: 11px;
  background: linear-gradient(145deg, rgba(39, 139, 255, .28), rgba(25, 211, 197, .08));
  box-shadow: inset 0 0 16px rgba(39, 139, 255, .2), 0 0 24px rgba(39, 139, 255, .14);
  transform: rotate(45deg);
}
.brand-mark::before, .brand-mark span {
  content: '';
  width: 13px;
  height: 13px;
  border: 2px solid #5bdcff;
  border-radius: 4px;
}
.brand-mark span { position: absolute; width: 5px; height: 5px; border-color: #a5f3fc; }
.brand-copy { display: grid; text-align: left; }
.brand-copy b { font-size: 21px; letter-spacing: .02em; }
.brand-copy small { margin-top: 1px; color: #6f99c5; font-size: 8px; font-weight: 700; letter-spacing: .18em; }
.nav-actions, .system-badge { display: flex; align-items: center; }
.nav-actions { gap: 22px; }
.system-badge { gap: 8px; color: #a8c1db; font-size: 12px; }
.system-badge i { width: 7px; height: 7px; border-radius: 50%; background: #26e0b3; box-shadow: 0 0 0 5px rgba(38, 224, 179, .1), 0 0 14px rgba(38, 224, 179, .7); }
.nav-button { height: 38px; padding-inline: 20px; border-color: rgba(130, 184, 240, .35); color: #eaf6ff; background: rgba(5, 20, 47, .36); }
.hero {
  position: relative;
  display: flex;
  width: min(1440px, calc(100% - 96px));
  min-height: calc(100vh - 278px);
  margin: 0 auto;
  align-items: center;
}
.hero-copy { width: min(630px, 47vw); padding: 58px 0 72px; }
.eyebrow { display: flex; align-items: center; gap: 10px; color: #4edee1; font-size: 10px; font-weight: 800; letter-spacing: .2em; }
.eyebrow i { width: 3px; height: 3px; border-radius: 50%; background: #398dff; }
h1 { margin: 22px 0 0; color: #f7fbff; font-size: clamp(46px, 4.4vw, 72px); font-weight: 720; line-height: 1.12; letter-spacing: -.045em; text-shadow: 0 4px 36px rgba(0, 16, 44, .7); }
h1 em { color: transparent; background: linear-gradient(92deg, #70e8ec 0%, #58b7ff 52%, #9187ff 100%); background-clip: text; -webkit-background-clip: text; font-style: normal; }
.lead { max-width: 590px; margin: 25px 0 0; color: #a9bfd7; font-size: 17px; line-height: 1.85; letter-spacing: .015em; }
.actions { display: flex; gap: 14px; margin-top: 34px; }
.primary-action, .secondary-action { height: 50px; padding-inline: 24px; border-radius: 8px; font-size: 15px; font-weight: 650; }
.primary-action { border: 0; background: linear-gradient(100deg, #1677ff, #246cff 55%, #6954f5); box-shadow: 0 12px 34px rgba(30, 105, 255, .32); }
.primary-action:hover { background: linear-gradient(100deg, #3690ff, #3b7cff 55%, #816ef9) !important; transform: translateY(-1px); }
.secondary-action { border-color: rgba(130, 184, 240, .34); color: #e5f3ff; background: rgba(7, 23, 53, .5); backdrop-filter: blur(12px); }
.secondary-action:hover { color: #fff !important; border-color: rgba(91, 220, 255, .7) !important; background: rgba(16, 48, 91, .7) !important; }
.trust-row { display: flex; flex-wrap: wrap; gap: 20px; margin-top: 30px; color: #7795b6; font-size: 12px; }
.trust-row span { display: flex; align-items: center; gap: 6px; }
.trust-row :deep(.anticon) { color: #19d3c5; }
.visual-note { position: absolute; right: 2.5%; bottom: 19%; display: flex; align-items: center; gap: 12px; color: #d7ecff; text-shadow: 0 2px 12px #00132d; }
.note-line { width: 68px; height: 1px; background: linear-gradient(90deg, transparent, #64dce8); }
.visual-note div { display: grid; }
.visual-note small { color: #5bdde5; font-size: 9px; font-weight: 800; letter-spacing: .2em; }
.visual-note b { margin-top: 3px; font-size: 13px; }
.capabilities {
  position: relative;
  z-index: 2;
  display: grid;
  width: min(1440px, calc(100% - 96px));
  margin: 0 auto 28px;
  grid-template-columns: repeat(4, 1fr);
  border: 1px solid rgba(114, 176, 238, .17);
  border-radius: 16px;
  background: linear-gradient(100deg, rgba(6, 22, 49, .83), rgba(8, 29, 62, .62));
  box-shadow: 0 20px 50px rgba(0, 5, 18, .25);
  backdrop-filter: blur(18px);
}
.capability { display: flex; min-width: 0; align-items: center; gap: 13px; padding: 20px 24px; }
.capability + .capability { border-left: 1px solid rgba(114, 176, 238, .13); }
.capability-icon { display: grid; flex: 0 0 38px; width: 38px; height: 38px; place-items: center; border-radius: 10px; font-size: 18px; }
.capability-icon.cyan { color: #4ce4da; background: rgba(25, 211, 197, .1); }
.capability-icon.blue { color: #5aa4ff; background: rgba(39, 139, 255, .12); }
.capability-icon.violet { color: #9e8aff; background: rgba(124, 92, 252, .12); }
.capability-icon.amber { color: #ffbd62; background: rgba(245, 158, 11, .1); }
.capability div { display: grid; min-width: 0; }
.capability b { color: #eaf5ff; font-size: 14px; }
.capability small { margin-top: 4px; color: #6f8eaf; overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 1100px) {
  .backdrop { background-position: 60% center; }
  .ambient { background: linear-gradient(90deg, rgba(2, 10, 27, .98) 0%, rgba(2, 12, 31, .88) 42%, rgba(3, 18, 48, .3) 78%, rgba(2, 10, 26, .12) 100%), linear-gradient(180deg, rgba(0, 7, 20, .5), transparent 55%, rgba(0, 7, 19, .9)); }
  .hero-copy { width: min(590px, 62vw); }
  .visual-note { display: none; }
  .capabilities { grid-template-columns: repeat(2, 1fr); }
  .capability:nth-child(3) { border-left: 0; border-top: 1px solid rgba(114, 176, 238, .13); }
  .capability:nth-child(4) { border-top: 1px solid rgba(114, 176, 238, .13); }
}
@media (max-width: 720px) {
  .backdrop { background-position: 66% center; opacity: .68; }
  .ambient { background: linear-gradient(180deg, rgba(2, 10, 27, .88), rgba(2, 11, 29, .8) 43%, rgba(1, 8, 22, .97) 82%); }
  .nav, .hero, .capabilities { width: calc(100% - 36px); }
  .nav { height: 76px; }
  .brand-copy small, .system-badge { display: none; }
  .nav-actions { gap: 8px; }
  .nav-button { padding-inline: 14px; }
  .hero { min-height: auto; align-items: flex-start; }
  .hero-copy { width: 100%; padding: 76px 0 68px; }
  .eyebrow { font-size: 8px; }
  h1 { margin-top: 18px; font-size: clamp(39px, 12vw, 56px); }
  .lead { font-size: 15px; line-height: 1.75; }
  .actions { align-items: stretch; flex-direction: column; }
  .actions button { width: 100%; }
  .trust-row { gap: 11px 16px; }
  .capabilities { grid-template-columns: 1fr; }
  .capability + .capability { border-top: 1px solid rgba(114, 176, 238, .13); border-left: 0; }
}
</style>
