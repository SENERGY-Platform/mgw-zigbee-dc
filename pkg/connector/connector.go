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

package connector

import (
	"context"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	"log"
	"runtime/debug"
	"sync"
)

type Connector struct {
	config             configuration.Config
	eventbuffer        chan EventDesc
	commandbuffer      chan CommandDesc
	deviceupdatebuffer chan []model.ZigbeeDeviceInfo
	zigbee             *zigbee2mqtt.Client
	mgw                *mgw.Client
	devicerepo         DeviceRepo
}

type EventDesc struct {
	Device  model.ZigbeeDeviceInfo
	Payload []byte
}

type CommandDesc struct {
	DeviceId  string
	ServiceId string
	Command   mgw.Command
}

type DeviceRepo interface {
	FindDeviceTypeId(device model.ZigbeeDeviceInfo) (string, error)
}

func Start(ctx context.Context, wg *sync.WaitGroup, config configuration.Config) (connector *Connector, err error) {
	deviceRepo, err := devicerepo.New(ctx, config)
	if err != nil {
		return nil, err
	}
	return StartWithDependencies(ctx, wg, config, mgw.New, zigbee2mqtt.New, deviceRepo)
}

type ZigbeeProvider func(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, connector zigbee2mqtt.Connector) (*zigbee2mqtt.Client, error)
type MgwProvider func(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, refreshNotifier func()) (*mgw.Client, error)

func StartWithDependencies(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, mgw MgwProvider, zigbee ZigbeeProvider, devicerepo DeviceRepo) (connector *Connector, err error) {
	connector = &Connector{
		config:             config,
		eventbuffer:        make(chan EventDesc, 100),
		commandbuffer:      make(chan CommandDesc, 100),
		deviceupdatebuffer: make(chan []model.ZigbeeDeviceInfo, 100),
		devicerepo:         devicerepo,
	}
	connector.zigbee, err = zigbee(ctx, wg, config, connector)
	if err != nil {
		return
	}
	connector.mgw, err = mgw(ctx, wg, config, connector.NotifyRefresh)
	if err != nil {
		return
	}
	err = connector.mgw.ListenToCommands(connector.Command)
	if err != nil {
		return
	}
	err = connector.Start(ctx, wg)
	return
}

func (this *Connector) Event(device model.ZigbeeDeviceInfo, payload []byte) {
	this.eventbuffer <- EventDesc{
		Device:  device,
		Payload: payload,
	}
}

func (this *Connector) DeviceUpdate(devices []model.ZigbeeDeviceInfo) {
	this.deviceupdatebuffer <- devices
}

func (this *Connector) NotifyRefresh() {
	devices, err := this.zigbee.GetDeviceList(nil)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		return
	}
	this.DeviceUpdate(devices)
}

func (this *Connector) Command(deviceId string, serviceId string, command mgw.Command) {
	this.commandbuffer <- CommandDesc{
		DeviceId:  deviceId,
		ServiceId: serviceId,
		Command:   command,
	}
}

func (this *Connector) Start(ctx context.Context, wg *sync.WaitGroup) (err error) {
	err = this.startDeviceHandling(ctx, wg)
	if err != nil {
		return
	}
	err = this.startCommandHandling(ctx, wg)
	if err != nil {
		return
	}
	err = this.startEventHandling(ctx, wg)
	if err != nil {
		return
	}
	return nil
}
