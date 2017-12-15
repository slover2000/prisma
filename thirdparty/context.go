package thirdparty

import (
	"strings"
	"golang.org/x/net/context"
)

type SystemType = int

const (
	colonSep			= ":"

	// ThirdpartyDatabaseSystem define thirdparty system type
	TypeDatabaseSystem SystemType	= iota
	TypeCacheSystem
	TypeSearchSystem
	TypeUnknown
)

type interceptorKey struct{}

// DatabaseParam define database relative params
type DatabaseParam struct {
	System      string
	Database    string
	Table       string
	Action      string
	SQL         string
}

// CacheParam define cache relative params
type CacheParam struct {
	System      string
	Action      string
	Command     string
}

// SearchParam define search relative params
type SearchParam struct {
	System      string
	Action      string
	Index       string
	Document    string
	Command     string
}

// joinContextValue returns a derived context containing the values.
func joinContextValue(ctx context.Context, values ...string) context.Context {
	return context.WithValue(ctx, interceptorKey{}, strings.Join(values, colonSep))
}

// DetectContextValue detect which type contained in context
func DetectContextValue(ctx context.Context) SystemType {
	v := ctx.Value(interceptorKey{})
	if v != nil {
		switch v.(type) {
		case *DatabaseParam:
			return TypeDatabaseSystem
		case *CacheParam:
			return TypeCacheSystem
		case *SearchParam:
			return TypeSearchSystem
		}
	}

	return TypeUnknown
}

// JoinDatabaseContextValue returns a derived context containing the mongo values.
func JoinDatabaseContextValue(ctx context.Context, system, db, table, action, sql string) context.Context {
	param := &DatabaseParam{
		System: system,
		Database: db,
		Table: table,
		Action: action,
		SQL: sql,
	}
	return context.WithValue(ctx, interceptorKey{}, param)
}

// JoinCacheContextValue returns a derived context containing the mongo values.
func JoinCacheContextValue(ctx context.Context, system, action, command string) context.Context {
	param := &CacheParam{
		System: system,
		Action: action,
		Command: command,
	}
	return context.WithValue(ctx, interceptorKey{}, param)
}

// JoinSearchContextValue returns a derived context containing the mongo values.
func JoinSearchContextValue(ctx context.Context, system, index, action, command string) context.Context {
	param := &SearchParam{
		System: system,
		Index: index,
		Action: action,
		Command: command,
	}
	return context.WithValue(ctx, interceptorKey{}, param)
}

// parseContextValue parse string
func parseContextValue(ctx context.Context) ([]string, bool) {
	if s, ok := ctx.Value(interceptorKey{}).(string); ok {
		return strings.Split(s, colonSep), true
	}

	return nil, false
}

// ParseDatabaeContextValue parse thirdparty database params from context
func ParseDatabaeContextValue(ctx context.Context) (*DatabaseParam, bool) {
	param, ok := ctx.Value(interceptorKey{}).(*DatabaseParam)
	return param, ok
}

// ParseCacheContextValue parse thirdparty cache params from context
func ParseCacheContextValue(ctx context.Context) (*CacheParam, bool) {
	param, ok := ctx.Value(interceptorKey{}).(*CacheParam)
	return param, ok
}

// ParseSearchContextValue parse thirdparty search params from context
func ParseSearchContextValue(ctx context.Context) (*SearchParam, bool) {
	param, ok := ctx.Value(interceptorKey{}).(*SearchParam)
	return param, ok
}
