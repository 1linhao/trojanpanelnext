<template>
  <div v-if="formVisibleProps">
    <liquid-form-item
        :label="$t('table.xrayProtocol').toString()"
        prop="xrayProtocol"
    >
      <liquid-select
          v-model="nodeForm.xrayProtocol"
          controls-position="right"
          @change="xrayProtocolChange"
      >
        <option
            :label="item"
            :value="item"
            :key="index"
            v-for="(item, index) in xrayProtocols"
        ></option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayUotEnable').toString()"
        v-if="isXrayShadowsocks(nodeForm)"
    >
      <liquid-switch
          v-model="nodeForm.xrayUotEnable"
          :active-text="$t('table.enable').toString()"
          :inactive-text="$t('table.disable').toString()"
          :active-value="1"
          :inactive-value="0"
      />
      <div class="option-help ui-supporting-text">
        {{ $t('table.xrayUotHelp') }}
      </div>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayUotVersion').toString()"
        v-if="isXrayShadowsocks(nodeForm) && nodeForm.xrayUotEnable === 1"
    >
      <liquid-select v-model="nodeForm.xrayUotVersion">
        <option :value="1" label="1" />
        <option :value="2" label="2" />
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayXudpEnable').toString()"
        v-if="isXrayVless(nodeForm) || isXrayVmess(nodeForm)"
    >
      <liquid-switch
          v-model="nodeForm.xrayXudpEnable"
          :active-text="$t('table.enable').toString()"
          :inactive-text="$t('table.disable').toString()"
          :active-value="1"
          :inactive-value="0"
      />
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayMuxEnable').toString()"
        v-if="isXrayVless(nodeForm) || isXrayVmess(nodeForm) || isXrayTrojan(nodeForm)"
    >
      <liquid-switch
          v-model="nodeForm.xrayMuxEnable"
          :active-text="$t('table.enable').toString()"
          :inactive-text="$t('table.disable').toString()"
          :active-value="1"
          :inactive-value="0"
      />
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayStreamSettingsNetwork').toString()"
        prop="xrayStreamSettingsEntity.network"
        v-if="
        !(isXrayShadowsocksAEAD(nodeForm) || isXrayShadowsocks2022(nodeForm))
      "
    >
      <liquid-select
          v-model="nodeForm.xrayStreamSettingsEntity.network"
          controls-position="right"
          @change="xrayStreamSettingsNetworkChange"
      >
        <option
            :label="item"
            :value="item"
            :key="item"
            v-for="item in xrayStreamSettingsNetworks"
        ></option>
      </liquid-select>
    </liquid-form-item>

    <XrayFormWebSocket
        :form-visible-props="isXrayWs(nodeForm) && !isXrayShadowsocks(nodeForm)"
    />

    <liquid-form-item
        :label="$t('table.xrayStreamSettingsSecurity').toString()"
        prop="xrayStreamSettingsEntity.security"
        v-if="!isXrayShadowsocks(nodeForm)"
    >
      <liquid-select
          v-model="nodeForm.xrayStreamSettingsEntity.security"
          controls-position="right"
          @change="xrayStreamSettingsSecurityChange"
      >
        <option
            :label="item"
            :value="item"
            :key="item"
            v-for="item in xrayStreamSettingsSecuritys"
        ></option>
      </liquid-select>
    </liquid-form-item>

    <XrayFormTls
        :form-visible-props="isXrayStreamSettingsSecurityTls(nodeForm)"
    />

    <XrayFormReality
        :form-visible-props="isXrayStreamSettingsSecurityReality(nodeForm)"
    />

    <liquid-form-item
        :label="$t('table.xrayFlow').toString()"
        prop="xrayFlow"
        v-if="showXrayFlow(nodeForm)"
    >
      <liquid-select v-model="nodeForm.xrayFlow">
        <option
            :label="item"
            :value="item"
            :key="index"
            v-for="(item, index) in xrayFlows"
        ></option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xraySSMethod').toString()"
        prop="xraySSMethod"
        v-if="
        isXrayShadowsocks(nodeForm) ||
        isXrayShadowsocksAEAD(nodeForm) ||
        isXrayShadowsocks2022(nodeForm)
      "
    >
      <liquid-select v-model="nodeForm.xraySSMethod">
        <option
            :label="item"
            :value="item"
            :key="index"
            v-for="(item, index) in xraySSMethods"
        ></option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xraySSNetwork').toString()"
        prop="xraySettingsEntity.network"
        v-if="
        isXrayShadowsocksAEAD(nodeForm) ||
        isXrayShadowsocks2022(nodeForm)
      "
    >
      <liquid-select
          v-model="nodeForm.xraySettingsEntity.network"
          controls-position="right"
      >
        <option
            :label="item"
            :value="item"
            :key="index"
            v-for="(item, index) in xraySettingsNetworks"
        ></option>
      </liquid-select>
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xrayFallbacks').toString()"
        prop="xraySettingsEntity.fallbacks"
        v-if="showFallback(nodeForm)"
    >
      <liquid-tag
          v-for="(item, index) in nodeForm.xraySettingsEntity.fallbacks"
          :key="index"
          type="default"
          @close="deleteFallbackProps(item)"
          closable
          @click="handleFallbackDetailProps(item)"
      >
        {{ item.dest }}
      </liquid-tag>
      <liquid-button
          class="liquid-add-button"
          type="primary"
          size="sm"
          icon="plus"
          aria-label="添加回落配置"
          @click="handleCreateFallbackProps"
      ></liquid-button>
    </liquid-form-item>
    <liquid-form-item :label="$t('table.xraySocksUser').toString()" prop="xraySettingsEntity.accounts[0].user" v-if="isXraySocks(nodeForm)">
      <liquid-input v-model="nodeForm.xraySettingsEntity.accounts[0].user" />
    </liquid-form-item>
    <liquid-form-item :label="$t('table.xraySocksPass').toString()" prop="xraySettingsEntity.accounts[0].pass" v-if="isXraySocks(nodeForm)">
      <liquid-input v-model="nodeForm.xraySettingsEntity.accounts[0].pass" />
    </liquid-form-item>
    <liquid-form-item
        :label="$t('table.xraySocksUdp').toString()"
        prop="xraySettingsEntity.udp"
        v-if="isXraySocks(nodeForm)"
    >
      <liquid-switch
          v-model="nodeForm.xraySettingsEntity.udp"
          :active-text="$t('table.enable').toString()"
          :inactive-text="$t('table.disable').toString()"
          :active-value="true"
          :inactive-value="false"
      >
      </liquid-switch>
    </liquid-form-item>
  </div>
</template>

<script>
import {
  isXrayShadowsocks,
  isXrayShadowsocks2022,
  isXrayShadowsocksAEAD,
  isXraySocks,
  isXrayStreamSettingsSecurityReality,
  isXrayStreamSettingsSecurityTls, isXrayTrojan,
  isXrayVless,
  isXrayVmess,
  isXrayWs,
  showFallback,
  showXrayFlow
} from '@/utils/node.js'
import XrayFormReality from '@/views/node/list/components/XrayFormReality'
import XrayFormTls from '@/views/node/list/components/XrayFormTls'
import XrayFormWebSocket from '@/views/node/list/components/XrayFormWebSocket'

export default {
  name: 'XrayForm',
  components: {
    XrayFormWebSocket,
    XrayFormTls,
    XrayFormReality,
  },
  props: {
    formVisibleProps: {
      type: Boolean,
      require: true
    },
    handleCreateFallbackProps: {
      type: Function,
      required: true
    },
    handleFallbackDetailProps: {
      type: Function,
      required: true
    },
    deleteFallbackProps: {
      type: Function,
      required: true
    }
  },
  inject: {
    nodeForm: {
      from: 'nodeFormModel'
    }
  },
  computed: {
    xrayStreamSettingsSecuritys() {
      let securitys = ['none', 'tls']
      if (isXrayVless(this.nodeForm)) {
        securitys.push('reality')
      }
      if (isXrayTrojan(this.nodeForm)) {
        for (let i = 0; i < securitys.length; i++) {
          if (securitys[i] === 'none') {
            securitys.splice(i, 1)
          }
        }
      }
      return securitys
    },
    xrayFlows() {
      return ['none', 'xtls-rprx-vision']
    },
    xrayStreamSettingsNetworks() {
      let xrayStreamSettingsNetworks = [
        'tcp',
        // 'kcp',
        'ws'
        // 'http',
        // 'domainsocket',
        // 'quic',
        // 'grpc'
      ]
      if (this.isXrayShadowsocks(this.nodeForm) ||
          this.isXraySocks(this.nodeForm)) {
        return ['tcp']
      } else {
        return xrayStreamSettingsNetworks
      }
    }
  },
  data() {
    return {
      xraySettingsNetworks: ['tcp', 'udp', 'tcp,udp'],
      xraySSMethods: [
        'none',
        'aes-128-gcm',
        'aes-256-gcm',
        'chacha20-ietf-poly1305',
        'xchacha20-ietf-poly1305'
      ],
      xrayProtocols: [
        // 'dokodemo-door',
        // 'http',
        'vless',
        'vmess',
        'trojan',
        'shadowsocks',
        'socks'
      ]
    }
  },
  methods: {
    isXraySocks,
    isXrayShadowsocks,
    isXrayShadowsocksAEAD,
    isXrayShadowsocks2022,
    isXrayVless,
    isXrayVmess,
    isXrayTrojan,
    showFallback,
    showXrayFlow,
    isXrayWs,
    isXrayStreamSettingsSecurityTls,
    isXrayStreamSettingsSecurityReality,
    xrayProtocolChange() {
      if (isXrayVless(this.nodeForm)) {
        this.nodeForm.xrayStreamSettingsEntity.security = 'reality'
      } else if (isXrayShadowsocks(this.nodeForm) || isXraySocks(this.nodeForm)) {
        this.nodeForm.xrayStreamSettingsEntity.security = 'none'
        this.nodeForm.xrayStreamSettingsEntity.network = 'tcp'
      } else {
        this.nodeForm.xrayStreamSettingsEntity.security = 'tls'
      }
    },
    xrayStreamSettingsNetworkChange() {
      if (this.nodeForm.xrayStreamSettingsEntity.network === 'ws') {
        this.nodeForm.xrayStreamSettingsEntity.security = 'tls'
      }
    },
    xrayStreamSettingsSecurityChange() {
      if (
          isXrayVless(this.nodeForm) &&
          (this.nodeForm.xrayStreamSettingsEntity.security === 'tls' ||
              this.nodeForm.xrayStreamSettingsEntity.security === 'reality')
      ) {
        this.nodeForm.xrayFlow = 'xtls-rprx-vision'
      } else {
        this.nodeForm.xrayFlow = 'none'
      }
    }
  }
}
</script>

<style scoped>
.liquid-tag + .liquid-tag {
  margin-left: 10px;
}

.liquid-button {
  margin-left: 10px;
}

</style>
