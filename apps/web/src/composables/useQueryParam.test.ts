// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Ref } from 'vue'
import { defineComponent, h, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useQueryParam } from './useQueryParam'

describe('useQueryParam', () => {
  it('keeps every value when several parameters are set in one tick', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { render: () => null } }] })
    await router.push('/?a=1')
    let a!: Ref<string>, b!: Ref<string>, c!: Ref<number>
    mount(defineComponent({
      setup() {
        a = useQueryParam({ name: 'a', defaultValue: '' })
        b = useQueryParam({ name: 'b', defaultValue: '' })
        c = useQueryParam({ name: 'c', defaultValue: 5 })
        return () => h('div')
      },
    }), { global: { plugins: [router] } })
    a.value = 'x'
    b.value = 'y'
    c.value = 7
    expect(b.value).toBe('y') // visible before the navigation finishes
    await new Promise(r => setTimeout(r))
    await nextTick()
    expect(router.currentRoute.value.query).toEqual({ a: 'x', b: 'y', c: '7' })
    c.value = 5 // back to the default: removed
    await new Promise(r => setTimeout(r))
    expect(router.currentRoute.value.query).toEqual({ a: 'x', b: 'y' })
  })
})
