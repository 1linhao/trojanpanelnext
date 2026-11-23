// A computed property using Date.now() alone stays cached across expiry.
export default {
  data() {
    return { accountNow: Date.now() }
  },
  mounted() {
    this.updateAccountClock = () => { this.accountNow = Date.now() }
    this.accountClockTimer = setInterval(this.updateAccountClock, 1000)
    document.addEventListener('visibilitychange', this.updateAccountClock)
  },
  beforeDestroy() {
    clearInterval(this.accountClockTimer)
    document.removeEventListener('visibilitychange', this.updateAccountClock)
  }
}
