package bo

type QuotaStatus struct {
	ServerID      uint
	Period        string
	Mode          string
	UploadUsed    uint64
	DownloadUsed  uint64
	TotalLimit    uint64
	UploadLimit   uint64
	DownloadLimit uint64
	Reached       bool
}
