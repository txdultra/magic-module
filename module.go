/*
 *
 *  * Copyright (c) 2023-11 cdfsunrise.com
 *  *
 *  * Author: tangxd
 *  * Dep: Architecture team
 *
 *
 */

package module

import (
	"fmt"
	"github.com/ghodss/yaml"
	"gitlab.cdfsunrise.com/architect/magic-module/magic"
	"gitlab.cdfsunrise.com/architect/magic-module/utils"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var initOnce sync.Once

type Addr struct {
	Ip int32
}

func InitModules(configPath string) {
	//lib := dylib.NewLazyDLL("/Users/tangxd/Documents/GolandProjects/happy-fox-data-library-service/admin/goodscategory/rpc/open-symbol/lib_arm64_1.20.10.so")
	//lib := dylib.NewLazyDLL("/Users/tangxd/Documents/GolandProjects/happy-fox-data-library-service/admin/goodscategory/common/component/v1/lib_arm64_1.20.10.so")

	//InitM := lib.NewProc("ListenAddr")
	//str := "etc/category-rpc.yaml"

	//type A struct {
	//	ListenOn string
	//}
	//a := A{ListenOn: "main"}
	//a := "main"
	//InitM.Call(uintptr(unsafe.Pointer(&a)))
	//fmt.Println(a)

	//RunM := lib.NewProc("RunModuleListen")
	//go RunM.Call()
	//return
	//
	//RunL := lib.NewProc("RunModuleListen")
	//go RunL.Call(uintptr(unsafe.Pointer(&str)))
	//
	////load 2
	//lib2 := libs.NewLib("/Users/tangxd/Documents/GolandProjects/happy-fox-data-library-service/admin/goodscategory/rpc/open-symbol/lib2.so")
	//InitM2 := lib2.NewProc("InitM")
	//str2 := "etc/category-rpc.yaml"
	//InitM2.Call(uintptr(unsafe.Pointer(&str2)))
	//
	//RunL2 := lib2.NewProc("RunModuleListen")
	//go RunL2.Call(uintptr(unsafe.Pointer(&str)))

	//lib1 := magic.NewLib("../common/component/v1/lib.so")
	//lib2 := magic.NewLib("../common/component/v2/lib.so")
	//
	//Write1 := lib1.NewProc("Write")
	//Write2 := lib2.NewProc("Write")
	//Write1.Call()
	//Write2.Call()
	//
	//Read1 := lib1.NewProc("Read")
	//Read2 := lib2.NewProc("Read")
	//Read1.Call()
	//Read2.Call()
	//
	//return

	initOnce.Do(func() {
		modPath := "etc/modules.yaml"
		if configPath != "" {
			modPath = configPath
		}
		if _, err := os.Stat(modPath); err != nil {
			fmt.Println("modules.yaml not exist")
			return
		}

		content, err := os.ReadFile(modPath)
		if err != nil {
			panic(err)
		}
		var mCfg ModuleConfig
		err = yaml.Unmarshal(content, &mCfg)
		if err != nil {
			panic(err)
		}
		ModuleCfgs = mCfg

		for _, m := range mCfg.Modules {
			if m.Lazy {
				continue
			}
			loadMagicModuleWithConfig(m)
		}
	})
}

func loadMagicModuleWithConfig(m *Module) {
	fileName := filepath.Base(m.ModulePath)
	osModuleName := osModuleVersion(fileName)
	osModulePath := filepath.Join(filepath.Dir(m.ModulePath), osModuleName)
	loadMagicModule(
		osModulePath,
		m)
}

func loadMagicModule(pluginPath string, m *Module) {
	lib := magic.NewLib(pluginPath)

	initModuleFn := lib.NewProc("InitModule")
	config := utils.BuildModuleInfoToString(m.Name, m.Args, nil)
	_, _, err := initModuleFn.Call(uintptr(unsafe.Pointer(&config)))
	if err != nil {
		panic(err)
	}

	listenAddrFn := lib.NewProc("ListenAddr")
	var listenOn string
	listenAddrFn.Call(uintptr(unsafe.Pointer(&listenOn)))

	RegisterSrv(m.Name, m.ServiceKey, listenOn, lib, cShareType)

	if m.Run {
		runFn := lib.NewProc("RunModuleListen")
		go runFn.Call()
		if m.AfterLoadingWait > 0 {
			time.Sleep(time.Duration(m.AfterLoadingWait) * time.Millisecond)
		}
	}
}

func loadModule(pluginPath string, m *Module) {
	plu, err := plugin.Open(pluginPath)
	if err != nil {
		panic(err)
	}

	initModule, err := plu.Lookup("InitModule")
	if err != nil {
		panic(err)
	}
	initModuleFn := initModule.(func(map[string]string))
	initModuleFn(m.Args)

	srv, err := plu.Lookup("Symbol")
	if err != nil {
		panic(err)
	}

	//wr, err := plu.Lookup("Write")
	//wr.(func())()
	//rd, err := plu.Lookup("Read")
	//rd.(func())()

	serv := reflect.ValueOf(srv).Elem().Interface()
	RegisterSrv(m.Name, m.ServiceKey, "", serv, pluginType)

	// run
	if m.Run {
		run, err := plu.Lookup("Run")
		if err != nil {
			panic(err)
		}
		runFn := run.(func())
		go runFn()

		time.Sleep(1 * time.Second)
	}
}

func loadModuleWithName(name string) bool {
	for _, m := range ModuleCfgs.Modules {
		if m.Name == name {
			loadModuleWithConfig(m)
			return true
		}
	}
	return false
}

func loadModuleWithConfig(m *Module) {
	fileName := filepath.Base(m.ModulePath)
	osModuleName := osModuleVersion(fileName)
	osModulePath := filepath.Join(filepath.Dir(m.ModulePath), osModuleName)
	loadModule(
		osModulePath,
		m)
}

func osModuleVersion(fileName string) string {
	arch := runtime.GOARCH
	runtimeVersion := runtime.Version()
	goVersion := strings.Replace(runtimeVersion, "go", "", -1)
	withoutExt := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return fmt.Sprintf("%s_%s_%s.so", withoutExt, arch, goVersion)
}
