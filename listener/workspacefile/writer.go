package workspacefile

type Writer interface {
	Write(JSON []byte, clusterName string) error
}
