<template>
  <div v-if="formVisibleProps">
    <liquid-form-item
      :label="$t('table.hysteriaProtocol').toString()"
      prop="hysteriaProtocol"
    >
      <liquid-select
        v-model="nodeForm.hysteriaProtocol"
        :placeholder="$t('table.hysteriaProtocol').toString()"
        controls-position="right"
      >
        <option
          :label="item"
          :value="item"
          :key="item"
          v-for="item in hysteriaProtocols"
        ></option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaObfs').toString()"
      prop="hysteriaObfs"
    >
      <liquid-input v-model="nodeForm.hysteriaObfs" />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaUpMbps').toString()"
      prop="hysteriaUpMbps"
    >
      <liquid-number-input
        v-model.number="nodeForm.hysteriaUpMbps"
        controls-position="right"
        type="number"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaDownMbps').toString()"
      prop="hysteriaDownMbps"
    >
      <liquid-number-input
        v-model.number="nodeForm.hysteriaDownMbps"
        controls-position="right"
        type="number"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaServerName').toString()"
      prop="hysteriaServerName"
    >
      <liquid-input v-model="nodeForm.hysteriaServerName" />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaInsecure').toString()"
      prop="hysteriaInsecure"
    >
      <liquid-switch
        v-model="nodeForm.hysteriaInsecure"
        :active-text="$t('table.enable').toString()"
        :inactive-text="$t('table.disable').toString()"
        :active-value="1"
        :inactive-value="0"
      >
      </liquid-switch>
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteriaFastOpen').toString()"
      prop="hysteriaFastOpen"
    >
      <liquid-switch
        v-model="nodeForm.hysteriaFastOpen"
        :active-text="$t('table.enable').toString()"
        :inactive-text="$t('table.disable').toString()"
        :active-value="1"
        :inactive-value="0"
      >
      </liquid-switch>
    </liquid-form-item>
    <liquid-form-item>
      <aside class="form-hint">
        {{ $t('table.hysteriaTip') }}
      </aside>
    </liquid-form-item>
  </div>
</template>

<script>
export default {
  name: 'HysteriaForm',
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
  },
  data() {
    return {
      hysteriaProtocols: ['udp', 'wechat-video', 'faketcp']
    }
  }
}
</script>

<style scoped></style>
