package etcdv3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/naming"
)

// resolver is the implementaion of grpc.naming.Resolver
type resolver struct {
	serviceName string
}

// watcher is the implementaion of grpc.naming.Watcher
type watcher struct {
	resolver      resolver
	client        *etcd3.Client
	isInitialized bool
}

// NewResolver return resolver with service name
func NewResolver(serviceName string) *resolver {
	return &resolver{serviceName: serviceName}
}

func (r *resolver) Resolve() (naming.Watcher, error) {
	if len(r.serviceName) == 0 {
		return nil, errors.New("grpclb: no service name provided")
	}

	client, err := clientv3.New(etcd3.Config{
		Endpoints:   strings.Split(target, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("grpclb: creat etcd3 client failed: %s", err.Error())
	}
	defer cli.Close()

	return &watcher{resolver: r, client: client}, nil
}

// Close do nothing
func (w *watcher) Close() {
}

// Next to return the updates
func (w *watcher) Next() ([]*naming.Update, error) {
	// prefix is the etcd prefix/value to watch
	prefix := fmt.Sprintf("/%s/%s/", Prefix, w.re.serviceName)
	// check if is initialized
	if !w.isInitialized {
		// query addresses from etcd
		resp, err := w.client.Get(context.Background(), prefix, etcd3.WithPrefix())
		w.isInitialized = true
		if err == nil {
			addrs := extractAddrs(resp)
			//if not empty, return the updates or watcher new dir
			if l := len(addrs); l != 0 {
				updates := make([]*naming.Update, l)
				for i := range addrs {
					updates[i] = &naming.Update{Op: naming.Add, Addr: addrs[i]}
				}
				return updates, nil
			}
		}
	}
	// generate etcd Watcher
	rch := w.client.Watch(context.Background(), prefix, etcd3.WithPrefix())
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
func extractAddrs(resp *etcd3.GetResponse) []string {
	addrs := []string{}
	if resp == nil || resp.Kvs == nil {
		return addrs
	}
	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			addrs = append(addrs, string(v))
		}
	}
	return addrs
}
