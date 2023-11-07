/*
 *
 *  * Copyright (c) 2023-11 cdfsunrise.com
 *  *
 *  * Author: tangxd
 *  * Dep: Architecture team
 *
 *
 */

package txd

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"reflect"
	"strings"
	"sync"
)

type compType string

const (
	pluginType compType = "plugin"
	cShareType compType = "cshared"
)

type component struct {
	serviceKey string
	name       string
	typ        compType
	listenOn   string
	obj        any
}

var (
	components = make(map[string]*component)
	isRemote   = false
	l          sync.RWMutex
)

type RpcClient interface {
	Conn() *grpc.ClientConn
}

func RegisterSrv(name, serviceKey, listenOn string, obj any, typ compType) {
	l.Lock()
	defer l.Unlock()
	components[name] = &component{
		serviceKey: serviceKey,
		name:       name,
		obj:        obj,
		typ:        typ,
		listenOn:   listenOn,
	}
}

func getComponent(serviceKey string) *component {
	l.RLock()
	defer l.RUnlock()
	for _, c := range components {
		if c.serviceKey == serviceKey {
			return c
		}
	}
	return nil
}

func MagicWithDialOption(serviceKey string) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		protocol := "tcp"
		comp := getComponent(serviceKey)
		if comp != nil {
			tcpAddr, err := net.ResolveTCPAddr(protocol, comp.listenOn)
			if err != nil {
				panic(err)
			}
			newAddr := fmt.Sprintf("localhost:%d", tcpAddr.Port)
			return (&net.Dialer{}).DialContext(ctx, protocol, newAddr)
		}
		return (&net.Dialer{}).DialContext(ctx, protocol, addr)
	}
}

func LazyModuleFuncInvoke(name string, funcName string, args ...any) []any {
	l.Lock()
	defer l.Unlock()
	srv, ok := components[name]
	if !ok {
		loadPluginWithName(name)
	}

	srv = components[name]
	return invokeFunc(srv, funcName, args...)
}

func LazyModule(name string) any {
	l.Lock()
	defer l.Unlock()
	srv, ok := components[name]
	if !ok {
		loadPluginWithName(name)
		return components[name]
	}
	return srv
}

func invokeFunc(obj interface{}, methodName string, args ...any) []any {
	typ := reflect.TypeOf(obj)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Name == methodName {
			v := reflect.ValueOf(obj)
			m := v.MethodByName(methodName)
			var values []reflect.Value
			for _, arg := range args {
				values = append(values, reflect.ValueOf(arg))
			}
			vals := m.Call(values)
			var returnVals []any
			for _, val := range vals {
				returnVals = append(returnVals, val.Interface())
			}
			return returnVals
		}
	}

	return nil
}

func invoke(obj interface{}, methodName string, values []reflect.Value) []reflect.Value {
	typ := reflect.TypeOf(obj)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Name == methodName {
			v := reflect.ValueOf(obj)
			m := v.MethodByName(methodName)
			return m.Call(values)
		}
	}

	return nil
}

type ClientConnInterfaceWrapper struct {
	conn *grpc.ClientConn
}

func NewClientConnInterfaceWrapper(conn *grpc.ClientConn) *ClientConnInterfaceWrapper {
	return &ClientConnInterfaceWrapper{
		conn: conn,
	}
}

func NewRpcClientWrapper(cli RpcClient) *ClientConnInterfaceWrapper {
	return &ClientConnInterfaceWrapper{
		conn: cli.Conn(),
	}
}

func (h *ClientConnInterfaceWrapper) Conn() *grpc.ClientConn {
	return h.conn
}

func (h *ClientConnInterfaceWrapper) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return h.conn.NewStream(ctx, desc, method, opts...)
}

func (h *ClientConnInterfaceWrapper) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if isRemote {
		return h.conn.Invoke(ctx, method, args, reply, opts...)
	}

	l.RLock()
	defer l.RUnlock()

	ins := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(args),
	}
	mPath := strings.Split(method, "/")
	pkgNamespace := strings.Split(mPath[1], ".")
	pkgName := pkgNamespace[0]
	srv, ok := components[pkgName]
	if !ok {
		return errors.New("service name not exist")
	}

	values := invoke(srv, mPath[2], ins)
	val := values[0].Interface()
	err := values[1].Interface()

	v := reflect.ValueOf(reply)
	if v.Kind() == reflect.Ptr {
		elem := v.Elem()
		if elem.CanSet() {
			e := reflect.ValueOf(val)
			elem.Set(e.Elem())
		}
	}
	if err != nil {
		return err.(error)
	}
	return nil
}
