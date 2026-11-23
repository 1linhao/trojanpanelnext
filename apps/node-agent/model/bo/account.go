package bo

type AccountUpdateBo struct {
	Pass     string
	Hash     string
	Download int
	Upload   int
}

type AccountBo struct {
	ID       uint
	Username string
	Pass     string
	Hash     string
}
