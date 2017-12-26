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
type TypeFactory func() interface{}

// DataReceiver define interface method when a updating occur
type DataReceiver func(data interface{})

// ReaderOption define options of config reader
type ReaderOption func(*configReaderOptions)

// UsingMsgpackCodec enable msgpack codec
func UsingMsgpackCodec() ReaderOption {
	return func(c *configReaderOptions) { c.useMsgpackCodec = true }
}

// WithDialTimeout define dial timeout of connecting to etcd server
func WithDialTimeout(timeout time.Duration) ReaderOption {
	return func(c *configReaderOptions) { c.dialTimeout = timeout }
}

// NewConfigWatcher return a new instance of config watcher
func NewConfigWatcher(target, path string, factory TypeFactory, r DataReceiver, options ...ReaderOption) (Watcher, error) {
	configOptions := &configReaderOptions{
		dialTimeout: 5 * time.Second,
	}
	for _, option := range options {
		option(configOptions)
	}

	if factory == nil {
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
		factory:  factory,
		receiver: r,
	}
	if configOptions.useMsgpackCodec {
		watcher.codec = codec.NewMsgpackCodec()
	}

	// query config data from etcd
	ctx, cancel := context.WithTimeout(context.Background(), configOptions.dialTimeout)
	resp, err := watcher.client.Get(ctx, path)
	cancel()
	if err == nil {
		for i := range resp.Kvs {
			if v := resp.Kvs[i].Value; v != nil {
				data := watcher.factory()
				if err := watcher.codec.Unmarshal(v, data); err == nil {
					watcher.receiver(data)
				}
			}
		}
	}

	// star go routine to watch
	go func(w *configWatcher, watchPath string) {
		// watch the path
		for {
			rch := w.client.Watch(context.Background(), watchPath)
			for wresp := range rch {
				for _, ev := range wresp.Events {
					switch ev.Type {
					case mvccpb.PUT:
						data := watcher.factory()
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
	}(watcher, path)

	return watcher, nil
}

// Close close etcd v3 client
func (w *configWatcher) Close() {
	w.closed = true
	w.client.Close()
}
