package etcdv3

import (
	"log"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"encoding/json"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/naming"
)


// resolver is the implementaion of grpc.naming.Resolver
type etcdResolver struct {
	serviceName string
	groupName	string
	dialTimeout time.Duration
}

// watcher is the implementaion of grpc.naming.Watcher
type watcher struct {
	resolver      *etcdResolver
	client        *clientv3.Client
	isInitialized bool
}

// NewResolver return resolver with service name
func NewResolver(serviceName string, timeout time.Duration) *etcdResolver {
	return &etcdResolver{serviceName: serviceName, dialTimeout: timeout}
}

// NewResolver return resolver with service name
func NewResolverWithGroup(serviceName, groupName string, timeout time.Duration) *etcdResolver {
	return &etcdResolver{serviceName: serviceName, groupName: groupName, dialTimeout: timeout}
}

// @Title resolve the service from etcd, target is the dial address of etcd 
// @Description create a watcher of etcdv3
// @Param   key target    string  true    "the etcd address, like http://127.0.0.1:2379,http://127.0.0.1:12379,http://127.0.0.1:22379"
// @Success return an instance of watcher
// @Failure return error of etcd3
func (r *etcdResolver) Resolve(target string) (naming.Watcher, error) {
	if len(r.serviceName) == 0 {
		return nil, errors.New("grpclb: no service name provided")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(target, ","),
		DialTimeout: r.dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("grpclb: creat etcd3 client failed: %s", err.Error())
	}
	defer client.Close()

	return &watcher{resolver: r, client: client}, nil
}

// Close do nothing
func (w *watcher) Close() {

}

// Next to return the updates
func (w *watcher) Next() ([]*naming.Update, error) {
	// prefix is the etcd prefix/value to watch
	prefix := fmt.Sprintf("/%s/%s/", Prefix, w.resolver.serviceName)
	// check if is initialized
	if !w.isInitialized {
		// query addresses from etcd
		resp, err := w.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		if err == nil {
			w.isInitialized = true
			addrs := extractAddrs(resp, w.resolver.groupName)
			//if not empty, return the updates or watcher new dir
			if l := len(addrs); l != 0 {
				updates := make([]*naming.Update, l)
				for i, ep := range addrs {
					addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
					updates[i] = &naming.Update{Op: naming.Add, Addr: addr}
				}
				return updates, nil
			}
		} else {
			log.Printf("grpclb: get key with prefix[%s] failed:%s", prefix, err.Error())
		}
	}
	// generate etcd Watcher
	rch := w.client.Watch(context.Background(), prefix, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				return []*naming.Update{{Op: naming.Add, Addr: string(ev.Kv.Value)}}, nil
			case mvccpb.DELETE:
				return []*naming.Update{{Op: naming.Delete, Addr: string(ev.Kv.Value)}}, nil
			}
		}
	}
	return nil, nil
}

func extractAddrs(resp *clientv3.GetResponse, group string) []Endpoint {
	addrs := make([]Endpoint, 0)
	if resp == nil || resp.Kvs == nil {
		return addrs
	}

	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			ep := &Endpoint{}
			json.Unmarshal(v, ep)
			if len(group) > 0 && group != ep.Group {
				continue
			}
			addrs = append(addrs, *ep)
		}
	}
	return addrs
}
