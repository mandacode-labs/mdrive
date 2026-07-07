package content

type Content interface {
	Marshal() ([]byte, error)
}
