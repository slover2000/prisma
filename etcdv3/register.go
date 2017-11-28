package etcdv3

import (
	"fmt"
    "log"
    "strings"
    "time"
    "errors"
    "context"
	"encoding/json"
	
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
)

var stopSignal = make(chan bool, 1)
var client clientv3.Client
var serviceKey string

func initEndpointWithDefault(ep *Endpoint) {
    if len(ep.Group) == 0 {
        ep.Group = DefaultGroup
    }

    if ep.Weight == 0 {
        ep.Weight = DefaultWeight
    }
}

// Register register a service into etcdv3
func Register(serviceName string, target string, ep Endpoint, interval, ttl time.Duration) error {
    if len(ep.Host) == 0 {
        return errors.New("endpoint must have a host.")
    }

    if ep.Port == 0 {
        return errors.New("endpoint must have a port.")
    }
    initEndpointWithDefault(&ep)

	addressValue := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
	serviceKey = fmt.Sprintf("/%s/%s/%s", Prefix, serviceName, addressValue)
    endpointValue, _ := json.Marshal(&ep)

	// get endpoints for register dial address
	client, err := clientv3.New(clientv3.Config{
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
            resp, _ := client.Grant(context.TODO(), int64(ttl.Seconds()))
            // should get first, if not exist, set it
            _, err := client.Get(context.Background(), serviceKey)
            if err != nil {
                if err == rpctypes.ErrKeyNotFound {
                    if _, err := client.Put(context.TODO(), serviceKey, string(endpointValue), clientv3.WithLease(resp.ID)); err != nil {
                        log.Printf("grpclb: set service '%s' with ttl to etcd3 failed: %s", serviceName, err.Error())
                    }
                } else {
                    log.Printf("grpclb: service '%s' connect to etcd3 failed: %s", serviceName, err.Error())
                }
            } else {
                // refresh set to true for not notifying the watcher
                if _, err := client.Put(context.Background(), serviceKey, string(endpointValue), clientv3.WithLease(resp.ID)); err != nil {
                    log.Printf("grpclb: refresh service '%s' with ttl to etcd3 failed: %s", serviceName, err.Error())
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

// UnRegister delete registered service from etcd
func UnRegister() error {
    stopSignal <- true
    close(stopSignal)
    var err error;
    if _, err := client.Delete(context.Background(), serviceKey); err != nil {
        log.Printf("grpclb: deregister '%s' failed: %s", serviceKey, err.Error())
    } else {
        log.Printf("grpclb: deregister '%s' ok.", serviceKey)
    }
    return err
}