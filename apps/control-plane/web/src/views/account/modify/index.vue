<template>
  <div class="prototype-page">
    <ui-panel class="profile-card" motion-key="account-profile">
      <div class="card-head">
        <div>
          <span class="kicker">Account Security</span>
          <h2>个人资料</h2>
        </div>
      </div>
      <liquid-tabs
        id-prefix="profile"
        :tabs="profileTabs"
        v-model="activeName"
        label="个人资料"
      />
      <div :id="`profile-panel-${activeName}`" role="tabpanel" :aria-labelledby="`profile-tab-${activeName}`">
        <ModifyPass v-if="activeName === 'modify-pass'" />
        <ModifyProperty v-else-if="activeName === 'modify-property'" />
        <LiquidPalettePicker v-else inline />
      </div>
    </ui-panel>
  </div>
</template>

<script>
import ModifyPass from '@/views/account/modify/components/ModifyPass'
import ModifyProperty from '@/views/account/modify/components/ModifyProperty'
import LiquidPalettePicker from '@/components/LiquidPalettePicker'

export default {
  name: 'ProfilePage',
  components: { ModifyProperty, ModifyPass, LiquidPalettePicker },
  data() {
    return {
      activeName: 'modify-pass',
      profileTabs: [
        { value: 'modify-pass', label: '修改密码' },
        { value: 'modify-property', label: '账号属性' },
        { value: 'color-theme', label: '颜色主题' }
      ]
    }
  }
}
</script>

<style scoped></style>
