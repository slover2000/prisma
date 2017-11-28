package etcdv3

import (
	"fmt"
    "log"
    "strings"
	"time"
	"encoding/json"
	
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
)

var stopSignal = make(chan bool, 1)

// Register register a service into etcdv3
func Register(serviceName string, target string, ep Endpoint, interval time.Duration, ttl int) error {
	addressValue := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
	serviceKey = fmt.Sprintf("/%s/%s/%s", Prefix, serviceName, addressValue)

	endpointValue = json.Marshal(&ep)

	// get endpoints for register dial address
	var err error
	client, err := etcd3.New(etcd3.Config{
		Endpoints: strings.Split(target, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
        return fmt.Errorf("grpclb: create etcd3 client failed: %v", err)
    }

	go func() {
        // invoke self-register with ticker
        ticker := time.NewTicker(interval)
        for {
            // minimum lease TTL is ttl-second
            resp, _ := client.Grant(context.TODO(), int64(ttl))
            // should get first, if not exist, set it
            _, err := client.Get(context.Background(), serviceKey)
            if err != nil {
                if err == rpctypes.ErrKeyNotFound {
                    if _, err := client.Put(context.TODO(), serviceKey, serviceValue, etcd3.WithLease(resp.ID)); err != nil {
                        log.Printf("grpclb: set service '%s' with ttl to etcd3 failed: %s", name, err.Error())
                    }
                } else {
                    log.Printf("grpclb: service '%s' connect to etcd3 failed: %s", name, err.Error())
                }
            } else {
                // refresh set to true for not notifying the watcher
                if _, err := client.Put(context.Background(), serviceKey, serviceValue, etcd3.WithLease(resp.ID)); err != nil {
                    log.Printf("grpclb: refresh service '%s' with ttl to etcd3 failed: %s", name, err.Error())
                }
            }
            select {
            case <-stopSignal:
                return
            case <-ticker.C:
            }
        }
    }()
    return nil	
}