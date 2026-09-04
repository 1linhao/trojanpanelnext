<template>
  <span class="brand-mark panel-logo" aria-hidden="true">
    <img v-if="!failed" :key="branding.logoUrl" :src="branding.logoUrl" alt="" @error="failed = true" />
    <span v-else>{{ initial }}</span>
  </span>
</template>

<script>
import { panelBranding } from '@/utils/panel-branding'

export default {
  name: 'PanelLogo',
  data: () => ({ branding: panelBranding, failed: false }),
  computed: {
    initial() { return Array.from(this.branding.systemName)[0].toUpperCase() }
  },
  watch: {
    'branding.logoUrl'() { this.failed = false }
  }
}
</script>

<style scoped>
.panel-logo { flex-shrink: 0; overflow: hidden; }
.panel-logo img { display: block; width: 100%; height: 100%; object-fit: contain; }
</style>
