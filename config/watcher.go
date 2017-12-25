package config

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/slover2000/prisma/config/codec"
)

type configReaderOptions struct {
	useMsgpackCodec bool
	dialTimeout     time.Duration
	path            string
	factory         TypeFactory
}

type configWatcher struct {
	client   *clientv3.Client
	codec    codec.Codec
	factory  TypeFactory
	receiver DataReceiver
	closed   bool
}

// Watcher define config watcher interface
type Watcher interface {
	Close()
}

// TypeFactory define factory of data type
type TypeFactory interface {
	New() interface{}
}

// ReaderOption define options of config reader
type ReaderOption func(*configReaderOptions)

// DataReceiver define interface method when a updating occur
type DataReceiver func(data interface{})

func UsingMsgpackCodec() ReaderOption {
	return func(c *configReaderOptions) { c.useMsgpackCodec = true }
}

func WithConfigPath(path string) ReaderOption {
	return func(c *configReaderOptions) { c.path = path }
}

func WithDialTimeout(timeout time.Duration) ReaderOption {
	return func(c *configReaderOptions) { c.dialTimeout = timeout }
}

func RegisterTypeFactory(factory TypeFactory) ReaderOption {
	return func(c *configReaderOptions) { c.factory = factory }
}

func NewConfigWatcher(target string, r DataReceiver, options ...ReaderOption) (Watcher, error) {
	configOptions := &configReaderOptions{
		dialTimeout: 5 * time.Second,
	}
	for _, option := range options {
		option(configOptions)
	}

	if len(configOptions.path) == 0 {
		return nil, errors.New("watching path must be provided")
	}
	if configOptions.factory == nil {
		return nil, errors.New("data type factory must be provided")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(target, ","),
		DialTimeout: configOptions.dialTimeout,
	})
	if err != nil {
		return nil, err
	}

	watcher := &configWatcher{
		client:   client,
		codec:    codec.NewJSONCodec(), // default using json codec
		factory:  configOptions.factory,
		receiver: r,
	}
	if configOptions.useMsgpackCodec {
		watcher.codec = codec.NewMsgpackCodec()
	}

	// query config data from etcd
	watchPath := configOptions.path
	ctx, cancel := context.WithTimeout(context.Background(), configOptions.dialTimeout)
	resp, err := watcher.client.Get(ctx, watchPath)
	cancel()
	if err == nil {
		for i := range resp.Kvs {
			if v := resp.Kvs[i].Value; v != nil {
				data := watcher.factory.New()
				if err := watcher.codec.Unmarshal(v, data); err == nil {
					watcher.receiver(data)
				}
			}
		}
	}

	// star go routine to watch
	go func(w *configWatcher, path string) {
		// watch the path
		for {
			rch := w.client.Watch(context.Background(), path)
			for wresp := range rch {
				for _, ev := range wresp.Events {
					switch ev.Type {
					case mvccpb.PUT:
						data := watcher.factory.New()
						if err := w.codec.Unmarshal(ev.Kv.Value, data); err == nil {
							w.receiver(data)
						}
					}
				}
			}
			if w.closed {
				return
			}
		}
	}(watcher, watchPath)

	return watcher, nil
}

// Close close etcd v3 client
func (w *configWatcher) Close() {
	w.closed = true
	w.client.Close()
}
