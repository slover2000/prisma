package etcdv3

// Prefix is leading path of gprc service
const Prefix = "gprc-service"

// DefaultGroup is default group name
const DefaultGroup = "DEFAULT_GROUP"

// DefaultWeight is default weight of endpoint
const DefaultWeight = 5

// DefaultOpTimeout is default timeout for each etcd operation in seconds
const DefaultOpTimeout = 5

type EnvironmentType = int
// iota 初始化后会自动递增
const (
	Staging EnvironmentType = iota
	Preview
	Product
)

type (
	Endpoint struct {
		Host	string	`json:"host"`
		Port	int		`json:"port"`
		Weight	int		`json:"weight"`
		EnvType int		`json:"envtype"`
		Group 	string	`json:"group"`
	}
)