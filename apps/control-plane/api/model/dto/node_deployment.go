package dto

type NodeDeploymentDownloadDto struct {
	Id              uint   `json:"id"`
	WebHost         string `json:"webHost"`
	Email           string `json:"email"`
	CertificateMode string `json:"certificateMode"`
	CertificatePath string `json:"certificatePath"`
	PrivateKeyPath  string `json:"privateKeyPath"`
}
