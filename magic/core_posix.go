//go:build !windows && cgo
// +build !windows,cgo

/*
 *
 *  * Copyright (c) 2023-11 cdfsunrise.com
 *  *
 *  * Author: tangxd
 *  * Dep: Architecture team
 *
 *
 */

package magic

/*
	#cgo LDFLAGS: -ldl

	//#cgo LDFLAGS: -L. -Wl,-rpath,${SRCDIR} -llcl

	#include <dlfcn.h>
	#include <limits.h>
	#include <stdlib.h>
	#include <stdint.h>
	#include <stdio.h>

	static uintptr_t libOpen(const char* path) {
	     void* h = dlopen(path, RTLD_LAZY|RTLD_GLOBAL); // RTLD_LAZY RTLD_NOW |RTLD_GLOBAL
	     if (h == NULL) {
		     printf("cgo dlopen err: %s\n", (char*)dlerror());
	         return 0;
	     }
	     return (uintptr_t)h;
	}

	static uintptr_t libLookup(uintptr_t h, const char* name) {
	     void* r = dlsym((void*)h, name);
	     if (r == NULL) {
	         return 0;
	     }
	     return (uintptr_t)r;
	}

	static void libClose(uintptr_t h) {
	     if(h != 0) {
	         dlclose((void*)h);
	     }
	}

    static uint64_t Syscall0(void* addr) {
		return ((uint64_t(*)())addr)();
	}

    static uint64_t Syscall1(void* addr, void* p1) {
		return ((uint64_t(*)(void*))addr)(p1);
	}

    static uint64_t Syscall2(void* addr, void* p1, void* p2) {
		return ((uint64_t(*)(void*,void*))addr)(p1, p2);
	}

    static uint64_t Syscall3(void* addr, void* p1, void* p2, void* p3) {
		return ((uint64_t(*)(void*,void*,void*))addr)(p1, p2, p3);
	}

    static uint64_t Syscall4(void* addr, void* p1, void* p2, void* p3, void* p4) {
		return ((uint64_t(*)(void*,void*,void*,void*))addr)(p1, p2, p3, p4);
	}

    static uint64_t Syscall5(void* addr, void* p1, void* p2, void* p3, void* p4, void* p5) {
		return ((uint64_t(*)(void*,void*,void*,void*,void*))addr)(p1, p2, p3, p4, p5);
	}

*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

type Lib struct {
	mu        sync.Mutex
	handle    C.uintptr_t
	err       error
	Name      string
	mySyscall *FnProc
}

func NewLib(name string) *Lib {
	m := new(Lib)
	m.Name = name
	m.mu.Lock()
	defer m.mu.Unlock()

	cPath := (*C.char)(C.malloc(C.PATH_MAX + 1))
	defer C.free(unsafe.Pointer(cPath))

	cRelName := C.CString(m.libFullPath(name))
	defer C.free(unsafe.Pointer(cRelName))

	if C.realpath(cRelName, cPath) == nil {
		m.handle = C.libOpen(cRelName)
	} else {
		m.handle = C.libOpen(cPath)
	}
	if m.handle == 0 {
		m.err = fmt.Errorf("newLib dlopen(\"%s\") failed.", name)
		return m
	}
	m.mySyscall = m.NewProc("MyTxd")
	if m.mySyscall.Find() != nil {
		m.mySyscall = nil
	}

	return m
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func (l *Lib) libFullPath(name string) string {
	if runtime.GOOS == "darwin" {
		file, _ := exec.LookPath(os.Args[0])
		libPath := filepath.Dir(file) + "/" + name
		if fileExists(libPath) {
			return libPath
		}
	}
	return name
}

func (l *Lib) Load() error {
	return l.err
}

func (l *Lib) NewProc(name string) *FnProc {
	p := new(FnProc)
	p.Name = name
	p.lzdll = l
	return p
}

func (l *Lib) Close() {
	C.libClose(l.handle)
}

func (d *Lib) call(proc *FnProc, a ...uintptr) (r1, r2 uintptr, lastErr error) {
	if d.mySyscall == nil {
		return proc.CallOriginal(a...)
	}
	addr := proc.Addr()
	if addr != 0 {
		pLen := uintptr(len(a))
		switch pLen {
		case 0:
			return d.mySyscall.CallOriginal(addr, pLen)
		case 1:
			return d.mySyscall.CallOriginal(addr, pLen, a[0])
		case 2:
			return d.mySyscall.CallOriginal(addr, pLen, a[0], a[1])
		case 3:
			return d.mySyscall.CallOriginal(addr, pLen, a[0], a[1], a[2])
		case 4:
			return d.mySyscall.CallOriginal(addr, pLen, a[0], a[1], a[2], a[3])
		case 5:
			return d.mySyscall.CallOriginal(addr, pLen, a[0], a[1], a[2], a[3], a[4])
		default:
			panic("Call " + proc.Name + " with too many arguments " + strconv.Itoa(len(a)) + ".")
		}
	}
	return 0, 0, syscall.EINVAL
}

type FnProc struct {
	mu    sync.Mutex
	p     uintptr
	Name  string
	lzdll *Lib
}

func (p *FnProc) Addr() uintptr {
	err := p.Find()
	if err != nil {
		fmt.Println(err)
	}
	return p.p
}

func (p *FnProc) Find() error {
	if p.p == 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		cRelName := C.CString(p.Name)
		defer C.free(unsafe.Pointer(cRelName))
		p.p = uintptr(C.libLookup(p.lzdll.handle, cRelName))
	}
	if p.p == 0 {
		return errors.New("proc \"" + p.Name + "\" not find.")
	}
	return nil
}

func toPtr(a uintptr) unsafe.Pointer {
	return unsafe.Pointer(a)
}

func (p *FnProc) Call(a ...uintptr) (uintptr, uintptr, error) {
	return p.lzdll.call(p, a...)
}

func (p *FnProc) CallOriginal(a ...uintptr) (r1, r2 uintptr, lastErr error) {
	err := p.Find()
	if err != nil {
		fmt.Println(err)
		return 0, 0, syscall.EINVAL
	}
	var ret C.uint64_t
	switch len(a) {
	case 0:
		ret = C.Syscall0(unsafe.Pointer(p.p))
	case 1:
		ret = C.Syscall1(toPtr(p.p), toPtr(a[0]))
	case 2:
		ret = C.Syscall2(toPtr(p.p), toPtr(a[0]), toPtr(a[1]))
	case 3:
		ret = C.Syscall3(toPtr(p.p), toPtr(a[0]), toPtr(a[1]), toPtr(a[2]))
	case 4:
		ret = C.Syscall4(toPtr(p.p), toPtr(a[0]), toPtr(a[1]), toPtr(a[2]), toPtr(a[3]))
	case 5:
		ret = C.Syscall5(toPtr(p.p), toPtr(a[0]), toPtr(a[1]), toPtr(a[2]), toPtr(a[3]), toPtr(a[4]))

	default:
		panic("Call " + p.Name + " with too many arguments " + strconv.Itoa(len(a)) + ".")
	}
	return uintptr(ret), uintptr(ret >> 32), nil
}
