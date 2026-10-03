<template>
  <div class="trend-chart">
    <div class="trend-legend">
      <span v-for="s in series" :key="s.name" class="legend-item">
        <i class="legend-dot" :style="{ background: s.color }"></i>{{ s.name }}
        <b class="tabular-nums">{{ sum(s.data) }}</b>
      </span>
    </div>
    <div class="trend-svg-wrap" @mouseleave="hover = -1">
      <svg :viewBox="`0 0 ${W} ${H}`" class="trend-svg">
        <defs>
          <linearGradient v-for="(s, i) in series" :key="'g' + i" :id="`trend-grad-${uid}-${i}`" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" :stop-color="s.color" stop-opacity="0.22" />
            <stop offset="100%" :stop-color="s.color" stop-opacity="0" />
          </linearGradient>
        </defs>
        <!-- 横向网格线 -->
        <line v-for="t in ticks" :key="'t' + t" :x1="PL" :x2="W - PR" :y1="y(t)" :y2="y(t)" class="grid-line" />
        <text v-for="t in ticks" :key="'tl' + t" :x="PL - 8" :y="y(t) + 4" class="axis-text" text-anchor="end">{{ t }}</text>
        <!-- 悬停竖线 -->
        <line v-if="hover >= 0" :x1="x(hover)" :x2="x(hover)" :y1="PT" :y2="H - PB" class="hover-line" />
        <!-- 面积 + 折线 -->
        <g v-for="(s, i) in series" :key="'s' + i">
          <path :d="areaPath(s.data)" :fill="`url(#trend-grad-${uid}-${i})`" />
          <path :d="linePath(s.data)" fill="none" :stroke="s.color" stroke-width="2.2" stroke-linejoin="round" stroke-linecap="round" />
          <circle v-for="(v, j) in s.data" :key="j" :cx="x(j)" :cy="y(v)" :r="hover === j ? 4.5 : 3"
            :fill="hover === j ? s.color : '#fff'" :stroke="s.color" stroke-width="2" />
        </g>
        <!-- X 轴标签 -->
        <text v-for="(m, j) in labels" :key="'x' + j" :x="x(j)" :y="H - 8" class="axis-text" text-anchor="middle">{{ m }}</text>
        <!-- 悬停热区 -->
        <rect v-for="(m, j) in labels" :key="'h' + j" :x="x(j) - step / 2" :y="0" :width="step" :height="H"
          fill="transparent" @mouseenter="hover = j" @click="hover = j" />
      </svg>
      <div v-if="hover >= 0" class="trend-tip" :style="tipStyle">
        <div class="tip-title">{{ fullLabels[hover] || labels[hover] }}</div>
        <div v-for="s in series" :key="s.name" class="tip-row">
          <i class="legend-dot" :style="{ background: s.color }"></i>{{ s.name }}
          <b class="tabular-nums">{{ s.data[hover] }}</b>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  labels: { type: Array, default: () => [] },       // X 轴短标签
  fullLabels: { type: Array, default: () => [] },   // 悬停提示中的完整标签
  series: { type: Array, default: () => [] }        // [{ name, color, data: [] }]
})

const uid = Math.random().toString(36).slice(2, 8)
const W = 640, H = 220, PL = 36, PR = 16, PT = 16, PB = 30
const hover = ref(-1)

const n = computed(() => Math.max(props.labels.length, 1))
const step = computed(() => (W - PL - PR) / Math.max(n.value - 1, 1))
const maxVal = computed(() => {
  const m = Math.max(0, ...props.series.flatMap(s => s.data))
  if (m <= 4) return 4
  const pow = Math.pow(10, Math.floor(Math.log10(m)))
  return Math.ceil(m / pow) * pow
})
const ticks = computed(() => [0, 1, 2, 3, 4].map(i => Math.round((maxVal.value / 4) * i)))

const x = (j) => (n.value === 1 ? (PL + W - PR) / 2 : PL + j * step.value)
const y = (v) => PT + (H - PT - PB) * (1 - v / maxVal.value)
const linePath = (d) => d.map((v, j) => `${j ? 'L' : 'M'}${x(j)},${y(v)}`).join(' ')
const areaPath = (d) => d.length ? `${linePath(d)} L${x(d.length - 1)},${H - PB} L${x(0)},${H - PB} Z` : ''
const sum = (d) => d.reduce((a, b) => a + b, 0)

const tipStyle = computed(() => {
  const pct = (x(hover.value) / W) * 100
  return pct > 60 ? { right: `${100 - pct + 2}%` } : { left: `${pct + 2}%` }
})
</script>

<style scoped>
.trend-legend {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--yx-text-2);
  margin-bottom: 8px;
}
.legend-item b {
  margin-left: 6px;
  color: var(--yx-text-1);
  font-size: 15px;
}
.legend-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: 1px;
}
.trend-svg-wrap {
  position: relative;
}
.trend-svg {
  width: 100%;
  height: auto;
  display: block;
  overflow: visible;
}
.grid-line {
  stroke: var(--el-border-color-lighter);
  stroke-dasharray: 3 4;
}
.hover-line {
  stroke: var(--yx-gold);
  stroke-width: 1;
}
.axis-text {
  font-size: 11px;
  fill: var(--yx-text-3);
}
.trend-tip {
  position: absolute;
  top: 8px;
  background: rgba(27, 36, 51, 0.92);
  color: #fff;
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  pointer-events: none;
  min-width: 110px;
  box-shadow: var(--yx-shadow);
}
.tip-title {
  color: var(--yx-gold-light);
  margin-bottom: 4px;
}
.tip-row {
  display: flex;
  align-items: center;
  line-height: 1.8;
}
.tip-row b {
  margin-left: auto;
  padding-left: 12px;
}
</style>
