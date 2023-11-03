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

var ModuleCfgs ModuleConfig

type ModuleConfig struct {
	Modules []*Module
}

type Module struct {
	ModulePath       string
	Name             string
	Args             map[string]string
	Run              bool
	Lazy             bool
	ServiceKey       string
	AfterLoadingWait int
}
