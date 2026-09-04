<template>
  <div v-if="formVisibleProps">
    <liquid-form-item
      :label="$t('table.hysteria2ObfsPassword').toString()"
      prop="hysteria2ObfsPassword"
    >
      <liquid-input v-model="nodeForm.hysteria2ObfsPassword" />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2UpMbps').toString()"
      prop="hysteria2UpMbps"
    >
      <liquid-number-input
        v-model.number="nodeForm.hysteria2UpMbps"
        controls-position="right"
        type="number"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2DownMbps').toString()"
      prop="hysteria2DownMbps"
    >
      <liquid-number-input
        v-model.number="nodeForm.hysteria2DownMbps"
        controls-position="right"
        type="number"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2ServerName').toString()"
      prop="hysteria2ServerName"
    >
      <liquid-input v-model="nodeForm.hysteria2ServerName" />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2PortHopping').toString()"
      prop="hysteria2PortHopping"
    >
      <liquid-input
        v-model="nodeForm.hysteria2PortHopping"
        placeholder="30000-50000"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2HopInterval').toString()"
      prop="hysteria2HopInterval"
    >
      <liquid-number-input
        v-model.number="nodeForm.hysteria2HopInterval"
        controls-position="right"
        type="number"
        :min="5"
        :max="86400"
      />
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.hysteria2Insecure').toString()"
      prop="hysteria2Insecure"
    >
      <liquid-switch
        v-model="nodeForm.hysteria2Insecure"
        :active-text="$t('table.enable').toString()"
        :inactive-text="$t('table.disable').toString()"
        :active-value="1"
        :inactive-value="0"
      >
      </liquid-switch>
    </liquid-form-item>
  </div>
</template>

<script>
export default {
  name: 'Hysteria2Form',
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
    return {}
  }
}
</script>

<style scoped></style>
