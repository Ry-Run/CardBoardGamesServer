package remote

type Client interface {
	Run() error
	SendMsg(dst string, data []byte) error
	Close() error
}
