package workspacefile

type Reader interface {
	Read(clusterName string) ([]byte, error)
}
