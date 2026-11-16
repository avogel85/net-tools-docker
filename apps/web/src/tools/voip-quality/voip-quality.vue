<script setup lang="ts">
import type { Codec, Stats } from './voip-quality.service'
import { NAlert, NButton, NProgress, NTag } from 'naive-ui'
import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import LatencyChart from '@/components/LatencyChart.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CSelect from '@/ui/c-select.vue'
import { BUFFERS, CODECS, DURATIONS, packet, readPacket, stats } from './voip-quality.service'

const { t } = useI18n()
const codec = useQueryParam({ name: 'codec', defaultValue: 'g711' })
const duration = useQueryParam({ name: 'dur', defaultValue: 10 })
const buffer = useQueryParam({ name: 'buf', defaultValue: 40 })
const preset = computed(() => CODECS[(codec.value in CODECS ? codec.value : 'g711') as Codec])

const running = ref(false)
const progress = ref(0)
const error = ref('')
const rtts = shallowRef<(number | null)[]>([])
const result = shallowRef<Stats>()
let ws: WebSocket | undefined
let timer: ReturnType<typeof setInterval> | undefined
let grace: ReturnType<typeof setTimeout> | undefined

function stop() {
  clearInterval(timer)
  clearTimeout(grace)
  if (ws && ws.readyState <= WebSocket.OPEN)
    ws.close(1000)
  ws = undefined
  running.value = false
}
onBeforeUnmount(stop)

function start() {
  stop()
  error.value = ''
  result.value = undefined
  const total = Math.round(Math.min(20, duration.value) * 1000 / preset.value.intervalMs)
  const buf: (number | null)[] = Array.from<number | null>({ length: total }).fill(null)
  rtts.value = []
  progress.value = 0
  running.value = true
  let sent = 0
  let finished = false
  const finish = () => {
    if (finished)
      return
    finished = true
    rtts.value = [...buf]
    result.value = stats(buf, buffer.value)
    if (!result.value && !error.value)
      error.value = t('tools.voip-quality.noReplies')
    stop()
  }
  const socket = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/voip/ws`)
  socket.binaryType = 'arraybuffer'
  ws = socket
  socket.onopen = () => {
    timer = setInterval(() => {
      if (sent >= total) {
        clearInterval(timer)
        grace = setTimeout(finish, 2000) // wait for the last echoes
        return
      }
      socket.send(packet(sent, performance.now(), preset.value.bytes))
      sent++
      if (sent % 10 === 0) {
        progress.value = Math.round(sent / total * 100)
        rtts.value = buf.slice(0, sent)
      }
    }, preset.value.intervalMs)
  }
  socket.onmessage = (e: MessageEvent<ArrayBuffer>) => {
    const p = readPacket(e.data)
    if (p && p.seq < total && buf[p.seq] === null)
      buf[p.seq] = performance.now() - p.sentAt
  }
  socket.onclose = (e) => {
    if (finished)
      return
    if (e.code !== 1000)
      error.value = e.code === 1008 ? t('tools.voip-quality.limit') : t('tools.voip-quality.failed')
    finish()
  }
}

const rows = computed(() => {
  const s = result.value
  if (!s)
    return []
  const ms = (v: number) => `${v} ms`
  return [
    { key: t('tools.voip-quality.rows.packets'), value: t('tools.voip-quality.packetsValue', { sent: s.sent, received: s.received }) },
    { key: t('tools.voip-quality.rows.rtt'), value: `${ms(s.minRtt)} / ${ms(s.avgRtt)} / ${ms(s.p95Rtt)} / ${ms(s.maxRtt)}` },
    { key: t('tools.voip-quality.rows.oneWay'), value: `≈ ${ms(Math.round(s.avgRtt / 2))}` },
    { key: t('tools.voip-quality.rows.jitter'), value: ms(s.jitter) },
    { key: t('tools.voip-quality.rows.lost'), value: String(s.lost) },
    { key: t('tools.voip-quality.rows.late', { buffer: buffer.value }), value: String(s.late) },
    { key: t('tools.voip-quality.rows.loss'), value: `${s.lossPct}%` },
    { key: t('tools.voip-quality.rows.r'), value: String(s.r) },
  ]
})
const gradeType = { excellent: 'success', good: 'success', fair: 'warning', poor: 'error', bad: 'error' } as const
</script>

<template>
  <CCard>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-3">
      <CSelect v-model:value="codec" :label="t('tools.voip-quality.codec')" :options="Object.entries(CODECS).map(([value, c]) => ({ value, label: c.label }))" />
      <CSelect v-model:value="duration" :label="t('tools.voip-quality.duration')" :options="DURATIONS.map(d => ({ value: d, label: `${d} s` }))" />
      <CSelect v-model:value="buffer" :label="t('tools.voip-quality.buffer')" :options="BUFFERS.map(b => ({ value: b, label: `${b} ms` }))" />
    </div>
    <div class="flex flex-wrap items-center gap-3">
      <NButton type="primary" :loading="running" :disabled="running" data-testid="run" @click="start">
        {{ t('tools.voip-quality.run') }}
      </NButton>
      <NButton v-if="running" secondary @click="stop">
        {{ t('tools.voip-quality.stop') }}
      </NButton>
      <span class="text-sm opacity-75">{{ t('tools.voip-quality.rate', { pps: Math.round(1000 / preset.intervalMs), bytes: preset.bytes }) }}</span>
    </div>
    <NProgress v-if="running" type="line" :percentage="progress" :aria-label="t('tools.voip-quality.run')" class="mt-3" />
  </CCard>
  <NAlert v-if="error" type="error" class="mb-4" data-testid="error">
    {{ error }}
  </NAlert>
  <CCard v-if="rtts.length || result" :title="t('common.result')" data-testid="result">
    <div v-if="result" class="mb-3 flex flex-wrap items-center gap-4">
      <div>
        <div class="text-sm opacity-75">
          MOS
        </div>
        <div class="text-4xl font-semibold font-mono" data-testid="mos">
          {{ result.mos.toFixed(2) }}
        </div>
      </div>
      <NTag :type="gradeType[result.grade]" :bordered="false" size="large" data-testid="grade">
        {{ t(`tools.voip-quality.grades.${result.grade}`) }}
      </NTag>
    </div>
    <LatencyChart :samples="rtts" :label="t('tools.voip-quality.chart')" />
    <ResultTable v-if="result" :columns="[{ key: 'key', title: t('tools.voip-quality.metric'), width: 260 }, { key: 'value', title: t('tools.voip-quality.value'), mono: true, width: 260 }]" :rows="rows" filename="voip-quality.json" :label="t('common.result')" class="mt-3" />
  </CCard>
  <NAlert type="info" class="mb-4">
    {{ t('tools.voip-quality.tcpNote') }}
  </NAlert>
</template>
