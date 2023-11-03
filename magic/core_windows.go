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

import (
	"fmt"
	"strconv"
	"syscall"
)

type Lib struct {
	*syscall.LazyDLL
	mySyscall *syscall.LazyProc
}

func NewLib(name string) *Lib {
	l := new(Lib)
	l.LazyDLL = syscall.NewLazyDLL(name)
	if err := l.Load(); err != nil {
		return l
	}
	l.mySyscall = l.LazyDLL.NewProc("MyTxd")
	if l.mySyscall.Find() != nil {
		l.mySyscall = nil
	}
	return l
}

type FnProc struct {
	lzProc *syscall.LazyProc
	lzdll  *Lib
}

func (d *Lib) NewProc(name string) *FnProc {
	l := new(FnProc)
	l.lzProc = d.LazyDLL.NewProc(name)
	l.lzdll = d
	return l
}

func (d *Lib) Close() {
	if d.Handle() != 0 {
		syscall.FreeLibrary(syscall.Handle(d.Handle()))
	}
}

func (d *Lib) call(proc *FnProc, a ...uintptr) (r1, r2 uintptr, lastErr error) {
	if d.mySyscall == nil {
		return proc.CallOriginal(a...)
	}
	err := proc.Find()
	if err != nil {
		fmt.Println("proc \"" + proc.lzProc.Name + "\" not find.")
		return 0, 0, syscall.EINVAL
	}
	addr := proc.Addr()
	if addr != 0 {
		pLen := uintptr(len(a))
		switch pLen {
		case 0:
			return d.mySyscall.Call(addr, pLen)
		case 1:
			return d.mySyscall.Call(addr, pLen, a[0])
		case 2:
			return d.mySyscall.Call(addr, pLen, a[0], a[1])
		case 3:
			return d.mySyscall.Call(addr, pLen, a[0], a[1], a[2])
		case 4:
			return d.mySyscall.Call(addr, pLen, a[0], a[1], a[2], a[3])
		case 5:
			return d.mySyscall.Call(addr, pLen, a[0], a[1], a[2], a[3], a[4])
		default:
			panic("Call " + proc.lzProc.Name + " with too many arguments " + strconv.Itoa(len(a)) + ".")
		}
	}
	return 0, 0, syscall.EINVAL
}

func (p *FnProc) Addr() uintptr {
	return p.lzProc.Addr()
}

func (p *FnProc) Find() error {
	return p.lzProc.Find()
}

func (p *FnProc) Call(a ...uintptr) (r1, r2 uintptr, lastErr error) {
	return p.lzdll.call(p, a...)
}

func (p *FnProc) CallOriginal(a ...uintptr) (r1, r2 uintptr, lastErr error) {
	return p.lzProc.Call(a...)
}
