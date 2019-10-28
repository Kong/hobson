package e2e

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

const (
	checkPrefix = "hobson-test-check-"
)

var (
	resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", "127.0.0.1:5300")
		},
	}
)

func consulAddNodeToService(nodeName string, nodeAddress string,
	serviceName string, healthy bool) error {
	client, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		return err
	}

	cat := client.Catalog()

	nodeHealth := "critical"
	if healthy {
		nodeHealth = "passing"
	}

	// Don't add ID to CatalogRegistration since from 1.2.3 consul won't allow
	// rename a node with same name but different ID
	// https://github.com/hashicorp/consul/commit/ef3b81ab13bb022fa42186c0426eb6432042d21d
	reg := &capi.CatalogRegistration{
		Node:    nodeName,
		Address: nodeAddress,
		Service: &capi.AgentService{
			ID:      serviceName,
			Service: serviceName,
			Port:    12345,
			Tags:    nil,
		},
		Check: &capi.AgentCheck{
			CheckID: checkPrefix + nodeName + ":" + serviceName,
			Status:  nodeHealth,
		},
	}

	_, err = cat.Register(reg, nil)
	return err
}

func consulRemoveNode(nodeNames ...string) error {

	client, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		return err
	}

	cat := client.Catalog()

	for _, nodeName := range nodeNames {
		dereg := &capi.CatalogDeregistration{
			Node: nodeName,
		}
		_, err = cat.Deregister(dereg, nil)
		if err != nil {
			return nil
		}
	}

	return nil
}

func queryDNS(name string) ([]string, error) {
	// 200ms
	time.Sleep(time.Microsecond * 1000 * 200)

	ips, err := resolver.LookupIPAddr(context.Background(), name+".")
	var addr []string
	for _, ip := range ips {
		addr = append(addr, ip.String())
	}
	return addr, err
}

func TestSanity(t *testing.T) {
	assert := require.New(t)

	// empty records

	ips, err := queryDNS("aaa.foo")
	assert.Empty(ips)
	assert.Contains(fmt.Sprintf("%v", err), "no such host")
}

func TestSelectNode(t *testing.T) {
	assert := require.New(t)
	nodes := []string{}
	for i := 1; i < 10; i++ {
		nodes = append(nodes, fmt.Sprintf("node1-%d", i))
	}
	defer consulRemoveNode(nodes...)

	t.Run("unhealthy node doesn't return", func(t *testing.T) {
		err := consulAddNodeToService("node1-1", "10.0.0.1", "service1", false)
		assert.Nil(err)

		ips, err := queryDNS("service1.foo")
		assert.Contains(fmt.Sprintf("%v", err), "no such host")
		assert.Empty(ips, 0)
	})

	t.Run("healthy node returns", func(t *testing.T) {
		for i := 2; i < 8; i++ {
			err := consulAddNodeToService(
				fmt.Sprintf("node1-%d", i),
				fmt.Sprintf("10.0.0.%d", i),
				"service1", true)
			assert.Nil(err)
		}
		err := consulAddNodeToService("node1-2", "10.0.0.2", "service1", true)
		assert.Nil(err)
		ips, err := queryDNS("service1.foo")
		assert.Nil(err)
		assert.Len(ips, 1)
		assert.Contains(ips, "10.0.0.2")
	})

	t.Run("remove nodes with higher ip doesn't affect selection", func(t *testing.T) {
		for i := 6; i < 8; i++ {
			err := consulRemoveNode(fmt.Sprintf("node1-%d", i))
			assert.Nil(err)
		}
		time.Sleep(time.Second)
		ips, err := queryDNS("service1.foo")
		assert.Nil(err)
		assert.Len(ips, 1)
		assert.Contains(ips, "10.0.0.2")
	})

	t.Run("removed node out of rotation", func(t *testing.T) {
		err := consulRemoveNode("node1-2")
		assert.Nil(err)
		time.Sleep(time.Second)
		ips, err := queryDNS("service1.foo")
		assert.Nil(err)
		assert.Len(ips, 1)
		assert.Contains(ips, "10.0.0.3")
	})

	t.Run("unhealthy node out of rotation", func(t *testing.T) {
		err := consulAddNodeToService("node1-3", "10.0.0.3", "service1", false)
		assert.Nil(err)
		time.Sleep(time.Second)
		ips, err := queryDNS("service1.foo")
		assert.Nil(err)
		assert.Len(ips, 1)
		assert.Contains(ips, "10.0.0.4")
	})

}
