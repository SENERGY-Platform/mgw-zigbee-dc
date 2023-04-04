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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo/auth"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	"github.com/SENERGY-Platform/models/go/models"
	"log"
	"runtime/debug"
	"sync"
)

func Start(ctx context.Context, wg *sync.WaitGroup, config configuration.Config) (connector *Connector, err error) {
	deviceRepo, err := devicerepo.New(config, &auth.Auth{})
	if err != nil {
		return nil, err
	}
	return StartWithDependencies(ctx, wg, config, MgwFactoryCast(mgw.New), ZigbeeFactoryCast(zigbee2mqtt.New), deviceRepo)
}

type ZigbeeFactory func(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, connector zigbee2mqtt.Connector) (ZigbeeClient, error)
type MgwFactory func(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, refreshNotifier func()) (MgwClient, error)

func StartWithDependencies(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, mgwFactory MgwFactory, zigbee ZigbeeFactory, devicerepo DeviceRepo) (connector *Connector, err error) {
	connector = &Connector{
		config:             config,
		eventbuffer:        make(chan EventDesc, 100),
		commandbuffer:      make(chan CommandDesc, 100),
		deviceupdatebuffer: make(chan []model.ZigbeeDeviceInfo, 100),
		devicerepo:         devicerepo,
		devicestate:        map[string]mgw.State{},
	}
	connector.zigbee, err = zigbee(ctx, wg, config, connector)
	if err != nil {
		return
	}
	connector.mgw, err = mgwFactory(ctx, wg, config, connector.NotifyRefresh)
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

type Connector struct {
	config             configuration.Config
	eventbuffer        chan EventDesc
	commandbuffer      chan CommandDesc
	deviceupdatebuffer chan []model.ZigbeeDeviceInfo
	zigbee             ZigbeeClient
	mgw                MgwClient
	devicerepo         DeviceRepo
	devicestate        map[string]mgw.State
	devicestateMux     sync.Mutex
}

type ZigbeeClient interface {
	GetDeviceList(ctx context.Context) ([]model.ZigbeeDeviceInfo, error)
	GetByIeee(ieeeAddress string) ([]byte, error)
	SetByIeee(ieeeAddress string, pl []byte) error
	GetZigbeeMessageOutputStruct(device model.ZigbeeDeviceInfo) (req map[string]interface{})
	GetZigbeeMessageInputStruct(device model.ZigbeeDeviceInfo) (req map[string]interface{})
}

type MgwClient interface {
	ListenToCommands(commandHandler mgw.DeviceCommandHandler) error
	SendCommandError(commandId string, msg string)
	Respond(deviceId string, serviceId string, command mgw.Command) error
	SetDevice(deviceId string, deviceInfo mgw.DeviceInfo) error
	SendClientError(msg string)
	SendEvent(deviceId string, serviceId string, msg []byte) error
	SendDeviceError(localDeviceId string, message string)
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
	FindDeviceTypeId(device model.ZigbeeDeviceInfo) (dtId string, usedFallback bool, err error)
	CreateDeviceTypeWithDistinctAttributes(dt models.DeviceType, attributeKeys []string) (result models.DeviceType, code int, err error)
}

func (this *Connector) Event(device model.ZigbeeDeviceInfo, payload []byte) {
	this.eventbuffer <- EventDesc{
		Device:  device,
		Payload: payload,
	}
}

func (this *Connector) DeviceInfoUpdate(devices []model.ZigbeeDeviceInfo) {
	this.deviceupdatebuffer <- devices
}

func (this *Connector) NotifyRefresh() {
	devices, err := this.zigbee.GetDeviceList(nil)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		return
	}
	this.DeviceInfoUpdate(devices)
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
