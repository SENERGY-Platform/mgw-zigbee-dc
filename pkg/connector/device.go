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
	"github.com/SENERGY-Platform/models/go/models"
	"log"
	"reflect"
	"runtime/debug"
	"strconv"
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
		deviceTypeId, usedFallback, err := this.getDeviceTypeId(device)
		if errors.Is(err, model.NoMatchingDeviceTypeFound) {
			log.Println("WARNING: unable to find matching device type", err)
			if !usedFallback {
				if this.config.CreateMissingDeviceTypes {
					log.Println("create device type", err)
					deviceTypeId, err = this.createDeviceType(device)
					if err != nil {
						log.Println("WARNING: unable to create device type", err)
						return
					}
					//if no error: continue with mgw device state publish
				} else {
					missingDtMsg := this.getMissingDeviceTypeMessage(device)
					log.Println(missingDtMsg, "\n=============================")
					this.mgw.SendClientError(missingDtMsg)
					return
				}
			}

		} else if err != nil {
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

func (this *Connector) getDeviceTypeId(device model.ZigbeeDeviceInfo) (dtId string, usedFallback bool, err error) {
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

func (this *Connector) createDeviceType(device model.ZigbeeDeviceInfo) (string, error) {
	dt, _, err := this.devicerepo.CreateDeviceType(this.zigbeeDeviceToDeviceType(device))
	return dt.Id, err
}

func (this *Connector) zigbeeDeviceToDeviceType(device model.ZigbeeDeviceInfo) (dt models.DeviceType) {
	output := this.zigbee.GetZigbeeMessageOutputStruct(device)
	input := this.zigbee.GetZigbeeMessageInputStruct(device)
	vendor := device.Definition.Vendor
	m := device.Definition.Model
	return models.DeviceType{
		Name:          vendor + " " + m,
		DeviceClassId: this.config.CreateMissingDeviceTypesWithDeviceClass,
		Attributes: []models.Attribute{
			{Key: devicerepo.AttributeUsedForZigbee, Value: "true"},
			{Key: devicerepo.AttributeZigbeeVendor, Value: vendor},
			{Key: devicerepo.AttributeZigbeeModel, Value: m},
		},
		Services: []models.Service{
			{
				LocalId:     "set",
				Name:        "set",
				Interaction: models.REQUEST,
				ProtocolId:  this.config.CreateMissingDeviceTypesWithProtocol,
				Inputs: []models.Content{
					{
						ContentVariable:   valueToContentVariable("value", input),
						Serialization:     models.JSON,
						ProtocolSegmentId: this.config.CreateMissingDeviceTypesWithProtocolSegment,
					},
				},
			},
			{
				LocalId:     "get",
				Name:        "get",
				Interaction: models.EVENT_AND_REQUEST,
				ProtocolId:  this.config.CreateMissingDeviceTypesWithProtocol,
				Outputs: []models.Content{
					{
						ContentVariable:   valueToContentVariable("value", output),
						Serialization:     models.JSON,
						ProtocolSegmentId: this.config.CreateMissingDeviceTypesWithProtocolSegment,
					},
				},
			},
		},
	}
}

func valueToContentVariable(name string, data interface{}) (result models.ContentVariable) {
	result = models.ContentVariable{
		Name:                name,
		IsVoid:              false,
		SubContentVariables: nil,
	}
	switch v := data.(type) {
	case string:
		result.Type = models.String
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		result.Type = models.Float
	case bool:
		result.Type = models.Boolean
	case []interface{}:
		result.Type = models.List
		sub := []models.ContentVariable{}
		for i, e := range v {
			sub = append(sub, valueToContentVariable(strconv.Itoa(i), e))
		}
		result.SubContentVariables = sub
	case map[string]interface{}:
		result.Type = models.Structure
		sub := []models.ContentVariable{}
		for k, e := range v {
			sub = append(sub, valueToContentVariable(k, e))
		}
		result.SubContentVariables = sub
	default:
		result.Type = models.Type(reflect.TypeOf(v).String())
	}
	return result
}
