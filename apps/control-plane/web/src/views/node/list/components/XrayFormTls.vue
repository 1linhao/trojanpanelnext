<template>
  <div v-if="formVisibleProps">
    <liquid-form-item
      label="serverName"
      prop="xrayStreamSettingsEntity.tlsSettings.serverName"
    >
      <liquid-input
        v-model="nodeForm.xrayStreamSettingsEntity.tlsSettings.serverName"
      />
    </liquid-form-item>
    <liquid-form-item
      label="allowInsecure"
      prop="xrayStreamSettingsEntity.tlsSettings.allowInsecure"
    >
      <liquid-switch
        v-model="nodeForm.xrayStreamSettingsEntity.tlsSettings.allowInsecure"
        :active-text="$t('table.yes').toString()"
        :inactive-text="$t('table.no').toString()"
        :active-value="true"
        :inactive-value="false"
      >
      </liquid-switch>
    </liquid-form-item>
    <liquid-form-item label="alpn" prop="xrayStreamSettingsEntity.tlsSettings.alpn">
      <liquid-select
        v-model="nodeForm.xrayStreamSettingsEntity.tlsSettings.alpn"
        multiple
      >
        <option
          v-for="item in alpns"
          :key="item"
          :label="item"
          :value="item"
        >
        </option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
      :label="$t('table.fingerprint').toString()"
      prop="fingerprint"
    >
      <liquid-select
        v-model="nodeForm.xrayStreamSettingsEntity.tlsSettings.fingerprint"
        controls-position="right"
      >
        <option
          :label="item"
          :value="item"
          :key="index"
          v-for="(item, index) in fingerprints"
        ></option>
      </liquid-select>
    </liquid-form-item>
  </div>
</template>

<script>
import { fingerprints } from '@/utils/node'

export default {
  name: 'XrayFormTls',
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
      alpns: ['h2', 'http/1.1'],
      fingerprints
    }
  }
}
</script>

<style scoped></style>
