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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"log"
	"runtime/debug"
	"strings"
	"sync"
)

func (this *Connector) startDeviceHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("start device info handling")
		for {
			select {
			case <-ctx.Done():
				log.Println("stop device info handling")
				return
			case devices := <-this.deviceupdatebuffer:
				for _, device := range devices {
					this.handleDeviceInfoUpdate(device)
				}
			}
		}
	}()
	return nil
}

func (this *Connector) handleDeviceInfoUpdate(device model.ZigbeeDeviceInfo) {
	if device.Supported && device.InterviewCompleted {
		deviceId := this.getDeviceId(device)
		deviceName := this.getDeviceName(device)
		deviceTypeId, err := this.getDeviceTypeId(device)
		if errors.Is(err, model.NoMatchingDeviceTypeFound) {
			log.Println("WARNING: unable to find matching device type", err)
			missingDtMsg := this.getMissingDeviceTypeMessage(device)
			log.Println(missingDtMsg, "\n=============================")
			this.mgw.SendClientError(missingDtMsg)
			return
		}
		if err != nil {
			log.Println("ERROR:", err)
			debug.PrintStack()
			return
		}
		deviceState := this.getDeviceState(device)
		this.storeDeviceState(device.IeeeAddress, deviceState)
		err = this.mgw.SetDevice(deviceId, mgw.DeviceInfo{
			Name:       deviceName,
			State:      deviceState,
			DeviceType: deviceTypeId,
		})
		if err != nil {
			log.Println("ERROR:", err)
			debug.PrintStack()
			return
		}
	}
}

func (this *Connector) storeDeviceState(ieeeAddr string, state mgw.State) {
	this.devicestateMux.Lock()
	defer this.devicestateMux.Unlock()
	this.devicestate[ieeeAddr] = state
}

func (this *Connector) getStoredDeviceState(ieeeAddr string) (state mgw.State, ok bool) {
	this.devicestateMux.Lock()
	defer this.devicestateMux.Unlock()
	state, ok = this.devicestate[ieeeAddr]
	return
}

func (this *Connector) deviceIsOnline(ieeeAddress string) bool {
	state, _ := this.getStoredDeviceState(ieeeAddress)
	return state == mgw.Online
}

func (this *Connector) getDeviceId(device model.ZigbeeDeviceInfo) string {
	return this.config.DeviceIdPrefix + device.IeeeAddress
}

func (this *Connector) getDeviceIeeeAddress(deviceId string) (ieee string, prefixFound bool) {
	if strings.HasPrefix(deviceId, this.config.DeviceIdPrefix) {
		return strings.TrimPrefix(deviceId, this.config.DeviceIdPrefix), true
	}
	return "", false
}

func (this *Connector) getDeviceName(device model.ZigbeeDeviceInfo) string {
	return device.FriendlyName
}

func (this *Connector) getDeviceState(device model.ZigbeeDeviceInfo) mgw.State {
	if device.Disabled {
		return mgw.Offline
	} else {
		return mgw.Online
	}
}

func (this *Connector) getDeviceTypeId(device model.ZigbeeDeviceInfo) (string, error) {
	return this.devicerepo.FindDeviceTypeId(device)
}

func (this *Connector) getMissingDeviceTypeMessage(device model.ZigbeeDeviceInfo) string {
	return GetMissingDeviceTypeMessage(device, this.zigbee.GetZigbeeMessageOutputStruct, this.zigbee.GetZigbeeMessageInputStruct)
}

func GetMissingDeviceTypeMessage(device model.ZigbeeDeviceInfo, outputProvider func(device model.ZigbeeDeviceInfo) map[string]interface{}, inputProvider func(device model.ZigbeeDeviceInfo) map[string]interface{}) string {
	output, _ := json.MarshalIndent(outputProvider(device), "", "    ")
	input, _ := json.MarshalIndent(inputProvider(device), "", "    ")
	return fmt.Sprintf(`missing zigbee device-type, please provide a device-type with:
ref: https://www.zigbee2mqtt.io/devices/%v.html
attributes:
    - %v = true
    - %v = %v
    - %v = %v
services:
---------------
local-id: get
protocol: standard-connector
example output data:
%v
---------------
local-id: set
protocol: standard-connector
example input data:
%v
---------------
`, device.Definition.Model,
		devicerepo.AttributeUsedForZigbee,
		devicerepo.AttributeZigbeeVendor, device.Definition.Vendor,
		devicerepo.AttributeZigbeeModel, device.Definition.Model,
		string(output),
		string(input),
	)
}
