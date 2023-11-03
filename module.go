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
			magicLoadModuleWithConfig(m)
		}
	})
}

func magicLoadModuleWithConfig(m *Module) {
	fileName := filepath.Base(m.ModulePath)
	osModuleName := osModuleVersion(fileName)
	osModulePath := filepath.Join(filepath.Dir(m.ModulePath), osModuleName)
	magicLoadModule(
		osModulePath,
		m)
}

func magicLoadModule(pluginPath string, m *Module) {
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
