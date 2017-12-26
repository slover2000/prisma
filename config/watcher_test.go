package config

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/slover2000/prisma/config/codec"
)

type NestData struct {
	Addr string
	Port int
}

type MockData struct {
	FieldInt    int
	FieldString string
	FieldFloat  float32
	Nest        NestData
}

func NewMockType() interface{} {
	return &MockData{}
}

func MockDataReceiver(data interface{}) {
	if d, ok := data.(*MockData); ok {
		log.Printf("receive data:%v", d)
	}
}

func TestWatcher(t *testing.T) {
	etcdAddr := "http://10.98.16.215:2379"
	client, err := clientv3.New(clientv3.Config{
		Endpoints: strings.Split(etcdAddr, ","),
	})
	if err != nil {
		t.Errorf("can't connect to etcd server:%s", err.Error())
		return
	}
	// create mock data
	data := &MockData{
		FieldInt:    100,
		FieldFloat:  68.9,
		FieldString: "haaha",
		Nest:        NestData{Addr: "127.0.0.1", Port: 9900},
	}
	jsonCodec := codec.NewJSONCodec()
	b, _ := jsonCodec.Marshal(data)
	client.Put(context.Background(), "/config/data", string(b))

	watcher, err := NewConfigWatcher(etcdAddr, "/config/data", NewMockType, MockDataReceiver, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Errorf("create ectd watcher failed:%s", err.Error())
		return
	}

	time.Sleep(100 * time.Second)
	watcher.Close()
}
