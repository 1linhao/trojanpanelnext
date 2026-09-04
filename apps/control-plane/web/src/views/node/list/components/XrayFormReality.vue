<template>
  <div v-if="formVisibleProps">
    <liquid-form-item
      label="dest"
      prop="xrayStreamSettingsEntity.realitySettings.dest"
    >
      <liquid-input
        v-model="nodeForm.xrayStreamSettingsEntity.realitySettings.dest"
      />
    </liquid-form-item>
    <liquid-form-item
      label="xver"
      prop="xrayStreamSettingsEntity.realitySettings.xver"
    >
      <liquid-number-input
        v-model="nodeForm.xrayStreamSettingsEntity.realitySettings.xver"
        controls-position="right"
        type="number"
      />
    </liquid-form-item>
    <liquid-form-item
      label="serverNames"
      prop="xrayStreamSettingsEntity.realitySettings.serverNames"
    >
      <EditTag
        :dynamic-tags-props="
          nodeForm.xrayStreamSettingsEntity.realitySettings.serverNames
        "
        @update:dynamic-tags-props="
          nodeForm.xrayStreamSettingsEntity.realitySettings.serverNames = $event
        "
      />
    </liquid-form-item>

    <liquid-form-item
      :label="$t('table.fingerprint').toString()"
      prop="fingerprint"
    >
      <liquid-select
        v-model="nodeForm.xrayStreamSettingsEntity.realitySettings.fingerprint"
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

    <liquid-form-item label="publicKey" prop="realityPbk">
      <liquid-input v-model="nodeForm.realityPbk" />
    </liquid-form-item>
    <liquid-form-item
      label="privateKey"
      prop="xrayStreamSettingsEntity.realitySettings.privateKey"
    >
      <liquid-input
        v-model="nodeForm.xrayStreamSettingsEntity.realitySettings.privateKey"
      />
    </liquid-form-item>
    <liquid-form-item
      label="shortIds"
      prop="xrayStreamSettingsEntity.realitySettings.shortIds"
    >
      <EditTag
        :dynamic-tags-props="
          nodeForm.xrayStreamSettingsEntity.realitySettings.shortIds
        "
        @update:dynamic-tags-props="
          nodeForm.xrayStreamSettingsEntity.realitySettings.shortIds = $event
        "
        :value-can-empty="true"
      />
    </liquid-form-item>
    <liquid-form-item
      label="spiderX"
      prop="xrayStreamSettingsEntity.realitySettings.spiderX"
    >
      <liquid-input
        v-model="nodeForm.xrayStreamSettingsEntity.realitySettings.spiderX"
      />
    </liquid-form-item>
  </div>
</template>

<script>
import EditTag from '@/components/EditTag'
import { fingerprints } from '@/utils/node'

export default {
  name: 'XrayFormReality',
  components: {
    EditTag
  },
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
      fingerprints
    }
  }
}
</script>

<style scoped></style>
