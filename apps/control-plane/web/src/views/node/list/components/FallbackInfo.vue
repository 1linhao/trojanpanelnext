<template>
  <ui-dialog
    append-to-body
    :title="$t('table.detail').toString()"
    :visible="dialogVisibleProps"
    @close="$emit('update:dialogVisibleProps', false)"
    width="30%"
  >
    <liquid-form :model="fallbackPrpops" label-position="left">
      <liquid-form-item label="name">
        <liquid-tag>{{ fallbackPrpops.name }}</liquid-tag>
      </liquid-form-item>
      <liquid-form-item label="alpn">
        <liquid-tag>{{ fallbackPrpops.alpn }}</liquid-tag>
      </liquid-form-item>
      <liquid-form-item label="path">
        <liquid-tag>{{ fallbackPrpops.path }}</liquid-tag>
      </liquid-form-item>
      <liquid-form-item label="dest">
        <liquid-tag>{{ fallbackPrpops.dest }}</liquid-tag>
      </liquid-form-item>
      <liquid-form-item label="xver">
        <liquid-tag>{{ fallbackPrpops.xver }}</liquid-tag>
      </liquid-form-item>
    </liquid-form>
    <div slot="footer" class="dialog-footer">
      <liquid-button
        type="primary"
        @click="$emit('update:dialogVisibleProps', false)"
        >{{ $t('table.confirm') }}
      </liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
export default {
  name: 'FallbackInfo',
  props: {
    dialogVisibleProps: {
      type: Boolean,
      required: true
    },
    fallbackPrpops: {
      type: Object,
      required: true
    }
  }
}
</script>

<style scoped></style>
