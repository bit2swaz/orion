package cluster

import (
	"time"

	"github.com/hashicorp/memberlist"
)

type Cluster struct {
	list *memberlist.Memberlist
}

func New(conf *memberlist.Config) (*Cluster, error) {
	list, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		list: list,
	}, nil
}

func GetLifeguardConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()

	conf.AwarenessMaxMultiplier = 8

	conf.SuspicionMult = 4

	conf.RetransmitMult = 4

	conf.IndirectChecks = 3

	return conf
}

func (c *Cluster) Join(peers []string) (int, error) {
	return c.list.Join(peers)
}

func (c *Cluster) Leave(timeout time.Duration) error {
	return c.list.Leave(timeout)
}

func (c *Cluster) Members() []*memberlist.Node {
	return c.list.Members()
}
