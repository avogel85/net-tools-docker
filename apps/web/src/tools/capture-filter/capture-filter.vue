<script setup lang="ts">
import type { Direction, FilterInput, Proto, TcpFlags } from './capture-filter.service'
import { NAlert, NFormItem, NInputNumber, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { splitList } from '@/utils/ip'
import { build } from './capture-filter.service'

const { t } = useI18n()
const hosts = useQueryParam({ name: 'hosts', defaultValue: '' })
const direction = useQueryParam({ name: 'dir', defaultValue: 'any' })
const ports = useQueryParam({ name: 'ports', defaultValue: '' })
const proto = useQueryParam({ name: 'proto', defaultValue: '' })
const excludePorts = useQueryParam({ name: 'notports', defaultValue: '' })
const excludeHosts = useQueryParam({ name: 'nothosts', defaultValue: '' })
const vlan = useQueryParam({ name: 'vlan', defaultValue: '' })
const tcpFlags = useQueryParam({ name: 'flags', defaultValue: '' })
const minSize = useQueryParam({ name: 'size', defaultValue: 0 })
const iface = useQueryParam({ name: 'iface', defaultValue: 'eth0' })
const file = useQueryParam({ name: 'file', defaultValue: '' })

const input = computed<FilterInput>(() => ({
  hosts: splitList(hosts.value),
  direction: direction.value as Direction,
  ports: splitList(ports.value),
  proto: proto.value as Proto,
  excludePorts: splitList(excludePorts.value),
  excludeHosts: splitList(excludeHosts.value),
  vlan: vlan.value.trim(),
  tcpFlags: tcpFlags.value as TcpFlags,
  minSize: minSize.value,
  iface: iface.value.trim(),
  file: file.value.trim(),
}))
const r = computed(() => build(input.value))
const err = (code: string) => (r.value.errors.includes(code) ? { isValid: false, status: 'error' as const, message: t(`tools.capture-filter.errors.${code}`) } : undefined)
const protos = computed(() => ['', 'tcp', 'udp', 'icmp', 'icmp6', 'arp'].map(p => ({ value: p, label: p ? p.toUpperCase() : t('tools.capture-filter.anyProto') })))
const flags = computed(() => ['', 'syn', 'synack', 'rst'].map(f => ({ value: f, label: t(`tools.capture-filter.flags.${f || 'none'}`) })))
const sizeModel = computed({
  get: () => minSize.value,
  set: (v: number | null) => {
    minSize.value = v ?? 0
  },
})
const outputs = computed(() => [
  ['tcpdump', r.value.tcpdump],
  ['bpf', r.value.bpf || '—'],
  ['display', r.value.display || '—'],
  ['tshark', r.value.tshark],
] as const)
</script>

<template>
  <CCard>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="hosts" :label="t('tools.capture-filter.hosts')" placeholder="192.0.2.10, 198.51.100.0/24" monospace test-id="hosts" :validation="err('hosts')" />
      <NFormItem :label="t('tools.capture-filter.direction')">
        <NRadioGroup v-model:value="direction" name="direction">
          <NRadioButton v-for="d in ['any', 'src', 'dst']" :key="d" :value="d">
            {{ t(`tools.capture-filter.directions.${d}`) }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
      <CInputText v-model:value="ports" :label="t('tools.capture-filter.ports')" placeholder="443, 8000-8080" monospace test-id="ports" :validation="err('ports')" />
      <CSelect v-model:value="proto" :label="t('tools.capture-filter.proto')" :options="protos" />
      <CInputText v-model:value="excludeHosts" :label="t('tools.capture-filter.excludeHosts')" placeholder="10.0.0.5" monospace test-id="exclude-hosts" :validation="err('excludeHosts')" />
      <CInputText v-model:value="excludePorts" :label="t('tools.capture-filter.excludePorts')" placeholder="22" monospace test-id="exclude-ports" :validation="err('excludePorts')" />
      <CSelect v-model:value="tcpFlags" :label="t('tools.capture-filter.tcpFlags')" :options="flags" />
      <div class="grid grid-cols-2 gap-x-4">
        <CInputText v-model:value="vlan" :label="t('tools.capture-filter.vlan')" placeholder="20" monospace test-id="vlan" :validation="err('vlan')" />
        <NFormItem :label="t('tools.capture-filter.minSize')">
          <NInputNumber v-model:value="sizeModel" :min="0" :max="65535" :show-button="false" :input-props="{ 'aria-label': t('tools.capture-filter.minSize') }" />
        </NFormItem>
      </div>
      <CInputText v-model:value="iface" :label="t('tools.capture-filter.iface')" placeholder="eth0" monospace test-id="iface" :validation="err('iface')" />
      <CInputText v-model:value="file" :label="t('tools.capture-filter.file')" placeholder="capture.pcap" monospace test-id="file" :validation="err('file')" />
    </div>
  </CCard>
  <NAlert v-if="r.errors.length" type="warning" class="mb-4">
    {{ t('tools.capture-filter.fix') }}
  </NAlert>
  <CCard v-else :title="t('tools.capture-filter.result')" data-testid="result">
    <div v-for="[k, v] in outputs" :key="k" class="mb-3">
      <div class="mb-1 flex items-center gap-2">
        <span class="text-sm font-semibold">{{ t(`tools.capture-filter.outputs.${k}`) }}</span>
        <CopyButton :text="v" size="tiny" />
      </div>
      <pre class="m-0 overflow-x-auto whitespace-pre-wrap break-all text-xs font-mono" :data-testid="`out-${k}`">{{ v }}</pre>
    </div>
    <p class="mb-0 text-sm">
      {{ t('tools.capture-filter.hint') }}
    </p>
  </CCard>
</template>
