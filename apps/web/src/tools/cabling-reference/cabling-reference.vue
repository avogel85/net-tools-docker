<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { CONNECTORS, COPPER, FIBER, filterRows, OPTICS } from './cabling-reference.service'

const { t } = useI18n()
const q = useQueryParam({ name: 'q', defaultValue: '' })
const col = (key: string, width: number, mono = false) => ({ key, title: t(`tools.cabling-reference.cols.${key}`), width, mono })
const sections = computed(() => [
  { id: 'copper', rows: filterRows(COPPER, q.value), columns: [col('name', 170), col('bandwidth', 120), col('speeds', 360), col('notes', 320)] },
  { id: 'fiber', rows: filterRows(FIBER, q.value), columns: [col('name', 80), col('core', 220), col('colour', 160), col('bandwidth', 230), col('notes', 320)] },
  { id: 'optics', rows: filterRows(OPTICS, q.value), columns: [col('standard', 150, true), col('form', 140), col('wavelength', 190), col('fiber', 130), col('connector', 120), col('reach', 210)] },
  { id: 'connectors', rows: filterRows(CONNECTORS, q.value), columns: [col('name', 220), col('type', 240), col('use', 420)] },
])
</script>

<template>
  <CCard>
    <CInputText v-model:value="q" :label="t('tools.cabling-reference.search')" placeholder="10GBASE-SR, OM4, MPO, Cat6A…" test-id="search" />
    <p class="mb-0 mt-0 text-sm">
      {{ t('tools.cabling-reference.hint') }}
    </p>
  </CCard>
  <template v-for="s in sections" :key="s.id">
    <CCard v-if="s.rows.length" :title="t(`tools.cabling-reference.sections.${s.id}`)" :data-testid="`section-${s.id}`">
      <ResultTable :columns="s.columns" :rows="s.rows" :filename="`${s.id}.json`" :label="t(`tools.cabling-reference.sections.${s.id}`)" />
    </CCard>
  </template>
</template>
