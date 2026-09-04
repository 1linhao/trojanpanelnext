// One owner for paginated-list response freshness and loading state.
export default {
  data() {
    return { listRequestVersion: 0, listRequestActive: true }
  },
  beforeDestroy() {
    this.listRequestActive = false
    this.listRequestVersion += 1
  },
  methods: {
    beginListRequest() {
      const version = ++this.listRequestVersion
      this.listLoading = true
      this.listError = ''
      return version
    },
    ownsListRequest(version) {
      return this.listRequestActive && version === this.listRequestVersion
    },
    finishListRequest(version) {
      if (this.ownsListRequest(version)) this.listLoading = false
    }
  }
}
