<template>
  <div class="dashboard-container">
    <component :is="currentRole" />
  </div>
</template>

<script>
import adminDashboard from './admin'
import userDashboard from './user'
import { mapGetters } from 'vuex'

export default {
  name: 'DashboardPage',
  components: { adminDashboard, userDashboard },
  data() {
    return {
      currentRole: 'userDashboard'
    }
  },
  computed: {
    ...mapGetters(['roles'])
  },
  created() {
    if (this.roles.includes('admin') || this.roles.includes('sysadmin')) {
      this.currentRole = 'adminDashboard'
    }
  }
}
</script>

<style scoped></style>
