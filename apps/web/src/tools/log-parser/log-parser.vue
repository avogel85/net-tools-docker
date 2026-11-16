<script setup lang="ts">
import { NButton, NCollapse, NCollapseItem, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultTable from '@/components/ResultTable.vue'
import CCard from '@/ui/c-card.vue'
import { parseLog, summary } from './log-parser.service'

const { t } = useI18n()
// Log lines can hold private data: kept in memory only, never in the URL.
const text = ref('')
const parsed = computed(() => parseLog(text.value))
const rows = computed(() => parsed.value.map(p => ({ line: p.line, format: t(`tools.log-parser.formats.${p.format}`), ...summary(p) })))
const counts = computed(() => {
  const c = new Map<string, number>()
  for (const p of parsed.value) c.set(p.format, (c.get(p.format) ?? 0) + 1)
  return [...c]
})
const columns = computed(() => [
  { key: 'line', title: '#', width: 50 },
  { key: 'format', title: t('tools.log-parser.format'), width: 140 },
  { key: 'time', title: t('tools.log-parser.time'), mono: true, width: 170 },
  { key: 'source', title: t('tools.log-parser.source'), mono: true, width: 150 },
  { key: 'severity', title: t('tools.log-parser.severity'), width: 100 },
  { key: 'text', title: t('tools.log-parser.text'), width: 420 },
])
const EXAMPLE = [
  '<165>1 2026-11-17T09:14:15.003Z web01.example.com nginx 811 - - upstream timed out',
  'Nov 17 10:01:02 gw sshd[1234]: Failed password for root from 203.0.113.5 port 52211 ssh2',
  '<166>Nov 17 2026 10:00:00: %ASA-6-302013: Built inbound TCP connection 4711 for outside:203.0.113.5/52211 (203.0.113.5/52211) to inside:10.0.0.5/443 (198.51.100.10/443)',
  'date=2026-11-17 time=10:00:00 devname="FG-60F" type="traffic" subtype="forward" srcip=10.0.0.5 dstip=203.0.113.9 dstport=443 action="deny"',
  '203.0.113.5 - - [17/Nov/2026:10:00:00 +0100] "GET /index.html HTTP/1.1" 404 512 "-" "curl/8.0"',
  '{"level":"error","msg":"database unreachable","service":"api"}',
].join('\n')
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.log-parser.input')">
      <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 6, maxRows: 20 }" class="font-mono" :placeholder="t('tools.log-parser.placeholder')" :input-props="{ 'aria-label': t('tools.log-parser.input'), 'spellcheck': false }" data-testid="input" />
    </NFormItem>
    <div class="flex flex-wrap items-center gap-3">
      <NButton secondary size="small" data-testid="example" @click="text = EXAMPLE">
        {{ t('tools.log-parser.example') }}
      </NButton>
      <span class="text-sm opacity-80">{{ t('tools.log-parser.hint') }}</span>
    </div>
  </CCard>
  <template v-if="parsed.length">
    <CCard :title="t('tools.log-parser.overview', { n: parsed.length })" data-testid="result">
      <div class="mb-3 flex flex-wrap gap-2">
        <NTag v-for="[f, n] in counts" :key="f" size="small" :bordered="false">
          {{ t(`tools.log-parser.formats.${f}`) }}: {{ n }}
        </NTag>
      </div>
      <ResultTable :columns="columns" :rows="rows" filename="log-lines.json" :label="t('tools.log-parser.overview', { n: parsed.length })" />
    </CCard>
    <CCard :title="t('tools.log-parser.fields')">
      <NCollapse :default-expanded-names="[parsed[0]!.line]">
        <NCollapseItem v-for="p in parsed.slice(0, 100)" :key="p.line" :name="p.line" :title="`#${p.line} · ${t(`tools.log-parser.formats.${p.format}`)}`" data-testid="line">
          <dl class="grid grid-cols-1 m-0 gap-x-4 gap-y-1 text-sm md:grid-cols-[12rem_1fr]">
            <template v-for="([k, v], i) in p.fields" :key="i">
              <dt class="font-semibold font-mono">
                {{ k }}
              </dt>
              <dd class="m-0 break-all font-mono">
                {{ v }}
              </dd>
            </template>
          </dl>
        </NCollapseItem>
      </NCollapse>
    </CCard>
  </template>
</template>
