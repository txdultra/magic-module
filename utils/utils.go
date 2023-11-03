/*
 *
 *  * Copyright (c) 2023-11 cdfsunrise.com
 *  *
 *  * Author: tangxd
 *  * Dep: Architecture team
 *
 *
 */

package utils

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

func GetParam(ptr uintptr) (*ModuleParam, error) {
	str := (*string)(unsafe.Pointer(ptr))
	fmt.Println(*str)
	var info ModuleParam
	err := json.Unmarshal([]byte(*str), &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func SetListenOn(ptr uintptr, listenOn string) {
	str := (*string)(unsafe.Pointer(ptr))
	*str = listenOn
}

type ModuleParam struct {
	Name    string
	Args    map[string]string
	Extends map[string]any
}

func BuildModuleInfoToString(name string, args map[string]string, extends map[string]any) string {
	info := ModuleParam{
		name, args, extends,
	}
	data, _ := json.Marshal(&info)
	return string(data)
}
