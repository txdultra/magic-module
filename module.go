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
	osModuleName := osModuleVersion(fileName, false)
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
	_, _, err = listenAddrFn.Call(uintptr(unsafe.Pointer(&listenOn)))
	if err != nil {
		fmt.Println("ListenAddr method not call:" + err.Error())
	}

	copyAddr := listenOn
	RegisterSrv(m.Name, m.ServiceKey, copyAddr, lib, cShareType)

	if m.Run {
		runFn := lib.NewProc("Run")
		go runFn.Call()
		if m.AfterLoadingWait > 0 {
			time.Sleep(time.Duration(m.AfterLoadingWait) * time.Millisecond)
		}
	}
}

func loadPlugin(pluginPath, name, serviceKey string, args map[string]string, runListen bool) {
	plu, err := plugin.Open(pluginPath)
	if err != nil {
		panic(err)
	}

	initModule, err := plu.Lookup("InitModule")
	if err != nil {
		panic(err)
	}
	initModuleFn := initModule.(func(map[string]string))
	initModuleFn(args)

	srv, err := plu.Lookup("Symbol")
	if err != nil {
		panic(err)
	}

	//wr, err := plu.Lookup("Write")
	//wr.(func())()
	//rd, err := plu.Lookup("Read")
	//rd.(func())()

	serv := reflect.ValueOf(srv).Elem().Interface()
	RegisterSrv(name, serviceKey, "", serv, pluginType)

	// run
	if runListen {
		run, err := plu.Lookup("RunModuleListen")
		if err != nil {
			panic(err)
		}
		runFn := run.(func())
		go runFn()

		time.Sleep(1 * time.Second)
	}
}

func loadPluginWithName(name string) bool {
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
	osModuleName := osModuleVersion(fileName, true)
	osModulePath := filepath.Join(filepath.Dir(m.ModulePath), osModuleName)
	loadPlugin(
		osModulePath,
		m.Name,
		m.ServiceKey,
		m.Args,
		m.Run)
}

func osModuleVersion(fileName string, includeGoVersion bool) string {
	arch := runtime.GOARCH
	goos := runtime.GOOS

	runtimeVersion := runtime.Version()
	goVersion := strings.Replace(runtimeVersion, "go", "", -1)
	withoutExt := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	if !includeGoVersion {
		return fmt.Sprintf("%s_%s_%s.so", withoutExt, goos, arch)
	}
	return fmt.Sprintf("%s_%s_%s_%s.so", withoutExt, goos, arch, goVersion)
}
