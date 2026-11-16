<script setup lang="ts">
import type { Ref } from 'vue'
import type { Vendor } from './switch-cli.service'
import { NButton, NFormItem, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { buildGroups, groupText, IFACE_EXAMPLES, isValidDevice, isValidIface, VENDORS } from './switch-cli.service'

const { t } = useI18n()
const vendor = useQueryParam({ name: 'vendor', defaultValue: 'junos' }) as Ref<Vendor>
const device = useQueryParam({ name: 'device', defaultValue: '' })
const iface = useQueryParam({ name: 'if', defaultValue: '' })
const match = useQueryParam({ name: 'match', defaultValue: '' })

const deviceV = computed(() => device.value && !isValidDevice(device.value)
  ? { isValid: false, status: 'error' as const, message: t('tools.switch-cli.invalidDevice') }
  : undefined)
const ifaceV = computed(() => iface.value && !isValidIface(vendor.value, iface.value)
  ? { isValid: false, status: 'error' as const, message: t('tools.switch-cli.invalidIface', { examples: IFACE_EXAMPLES[vendor.value].slice(0, 3).join(', ') }) }
  : undefined)
const groups = computed(() => buildGroups(vendor.value, { device: device.value, iface: iface.value, match: match.value }).filter(g => g.commands.length))
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.switch-cli.vendor')" :show-feedback="false" class="mb-3">
      <NRadioGroup v-model:value="vendor" name="switch-vendor">
        <NRadioButton v-for="v in VENDORS" :key="v" :value="v" :label="t(`tools.switch-cli.vendors.${v}`)" />
      </NRadioGroup>
    </NFormItem>
    <div class="grid gap-x-4 sm:grid-cols-3">
      <CInputText v-model:value="device" :label="t('tools.switch-cli.device')" :placeholder="t('tools.switch-cli.devicePlaceholder')" :validation="deviceV" monospace test-id="device" />
      <CInputText v-model:value="iface" :label="t('tools.switch-cli.iface')" :placeholder="IFACE_EXAMPLES[vendor][0]" :validation="ifaceV" monospace test-id="iface" />
      <CInputText v-model:value="match" :label="t('tools.switch-cli.match')" :placeholder="t('tools.switch-cli.matchPlaceholder')" monospace test-id="match" />
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-xs nt-muted">{{ t('tools.switch-cli.examples') }}</span>
      <NButton v-for="ex in IFACE_EXAMPLES[vendor]" :key="ex" size="tiny" secondary @click="iface = ex">
        {{ ex }}
      </NButton>
    </div>
    <p class="mb-0 mt-3 text-xs nt-muted">
      {{ t('tools.switch-cli.note') }}
    </p>
  </CCard>
  <CCard v-for="g in groups" :key="g.id" :title="t(`tools.switch-cli.groups.${g.id}`)" :data-testid="`group-${g.id}`">
    <p v-if="g.id === 'optics'" class="mt-0 text-xs nt-muted">
      {{ t('tools.switch-cli.opticsHint') }}
    </p>
    <div class="flex flex-col gap-2">
      <div v-for="c in g.commands" :key="c.id" class="flex flex-wrap items-center gap-x-3 gap-y-1" :class="{ 'opacity-60': c.missing }">
        <div class="min-w-0 flex-1">
          <div class="text-xs nt-muted">
            {{ t(`tools.switch-cli.cmds.${c.id}`) }}
          </div>
          <code class="break-all text-sm font-mono" :data-testid="`cmd-${c.id}`">{{ c.text }}</code>
        </div>
        <CopyButton :text="c.text" size="tiny" />
      </div>
    </div>
    <div class="mt-3">
      <CopyButton :text="() => groupText(g)" :label="t('tools.switch-cli.copyGroup')" />
    </div>
  </CCard>
</template>
