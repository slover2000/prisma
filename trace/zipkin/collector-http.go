package zipkin

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/slover2000/prisma/trace/thrift-gen/zipkincore"
)