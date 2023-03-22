/*
 * Copyright (c) 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package zigbee2mqtt

import (
	"context"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"sync"
	"time"
)

type DeviceRegister struct {
	list           []model.ZigbeeDeviceInfo
	byIeee         map[string]model.ZigbeeDeviceInfo
	byFriendlyName map[string]model.ZigbeeDeviceInfo
	mux            sync.Mutex
	init           bool
	initWg         *sync.WaitGroup
}

func NewDeviceRegister() *DeviceRegister {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &DeviceRegister{
		list:           []model.ZigbeeDeviceInfo{},
		byIeee:         map[string]model.ZigbeeDeviceInfo{},
		byFriendlyName: map[string]model.ZigbeeDeviceInfo{},
		init:           false,
		initWg:         wg,
	}
}

func (this *DeviceRegister) waitForInit(ctx context.Context) error {
	if this.init {
		return nil
	}
	if ctx == nil {
		ctx, _ = context.WithTimeout(context.Background(), time.Minute)
	}
	timeout, cancel := context.WithCancel(ctx)
	go func() {
		this.initWg.Wait()
		cancel()
	}()
	<-timeout.Done()
	err := timeout.Err()
	if err == context.Canceled {
		return nil
	}
	return err
}

func (this *DeviceRegister) Set(devices []model.ZigbeeDeviceInfo) {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.list = devices
	this.byIeee = map[string]model.ZigbeeDeviceInfo{}
	this.byFriendlyName = map[string]model.ZigbeeDeviceInfo{}
	for _, e := range devices {
		this.byIeee[e.IeeeAddress] = e
		this.byFriendlyName[e.FriendlyName] = e
	}
	if !this.init {
		this.initWg.Done()
	}
}

// GetByIeee returns a model.ZigbeeDeviceInfo by its IeeeAddress
// ctx may be nil (defaults to context with 1 minute timeout)
// if no device is found err == model.ErrNotFound
// if the ctx is done before the register received its first Set() call, GetByIeee() returns the ctx error
// except if the ctx error is context.Canceled, then no error is returned
func (this *DeviceRegister) GetByIeee(ctx context.Context, ieeeAddress string) (result model.ZigbeeDeviceInfo, err error) {
	err = this.waitForInit(ctx)
	if err != nil {
		return
	}
	this.mux.Lock()
	defer this.mux.Unlock()
	var ok bool
	result, ok = this.byIeee[ieeeAddress]
	if !ok {
		err = model.ErrNotFound
	}
	return result, err
}

// GetByFriendlyName returns a model.ZigbeeDeviceInfo by its FriendlyName
// ctx may be nil (defaults to context with 1 minute timeout)
// if no device is found err == model.ErrNotFound
// if the ctx is done before the register received its first Set() call, GetByFriendlyName() returns the ctx error
// except if the ctx error is context.Canceled, then no error is returned
func (this *DeviceRegister) GetByFriendlyName(ctx context.Context, name string) (result model.ZigbeeDeviceInfo, err error) {
	err = this.waitForInit(ctx)
	if err != nil {
		return
	}
	this.mux.Lock()
	defer this.mux.Unlock()
	var ok bool
	result, ok = this.byFriendlyName[name]
	if !ok {
		err = model.ErrNotFound
	}
	return result, err
}

// List returns a list of model.ZigbeeDeviceInfo
// ctx may be nil (defaults to context with 1 minute timeout)
// if no device is found err == model.ErrNotFound
// if the ctx is done before the register received its first Set() call, List() returns the ctx error
// except if the ctx error is context.Canceled, then no error is returned
func (this *DeviceRegister) List(ctx context.Context) (result []model.ZigbeeDeviceInfo, err error) {
	err = this.waitForInit(ctx)
	if err != nil {
		return
	}
	this.mux.Lock()
	defer this.mux.Unlock()
	return this.list, nil
}
