<template>
  <div v-if="formVisibleProps">
    <liquid-form-item :label="$t('table.naiveUotEnable').toString()">
      <liquid-switch
        v-model="nodeForm.naiveUotEnable"
        :active-value="1"
        :inactive-value="0"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.naiveUotVersion').toString()"
      v-if="nodeForm.naiveUotEnable === 1"
    >
      <liquid-select v-model="nodeForm.naiveUotVersion">
        <option label="v2" :value="2" />
        <option label="v1" :value="1" />
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item>
      <aside class="form-hint">
        {{ $t('table.naiveproxyTip') }}
      </aside>
    </liquid-form-item>
  </div>
</template>

<script>
export default {
  name: 'NaiveProxyForm',
  inject: {
    nodeForm: {
      from: 'nodeFormModel'
    }
  },
  props: {
    formVisibleProps: {
      type: Boolean,
      require: true
    }
  }
}
</script>

<style scoped></style>
