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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/models/go/models"
)

// devices whose device type could not be read are handled again after this delay, doubled per
// failed attempt up to deviceRetryMaxDelay; a new device list resets it
const deviceRetryMinDelay = time.Second
const deviceRetryMaxDelay = time.Minute

func (this *Connector) startDeviceHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		this.config.GetLogger().Info("start device info handling")
		var failed []model.ZigbeeDeviceInfo
		var retry <-chan time.Time
		delay := deviceRetryMinDelay
		for {
			var recovered []model.ZigbeeDeviceInfo
			var err error
			wasFailing := len(failed) > 0
			select {
			case <-ctx.Done():
				this.config.GetLogger().Info("stop device info handling")
				return
			case devices := <-this.deviceupdatebuffer:
				// zigbee2mqtt sends the complete list, so it replaces the devices still to be retried
				failed, recovered, err = this.handleDeviceInfoUpdates(devices, failed)
				delay = deviceRetryMinDelay
			case <-retry:
				failed, recovered, err = this.handleDeviceInfoUpdates(failed, failed)
				delay = min(2*delay, deviceRetryMaxDelay)
			}
			if len(recovered) > 0 {
				this.refreshEventValues(wg, recovered)
			}
			retry = nil
			if len(failed) > 0 {
				this.config.GetLogger().Warn("unable to read device types from device repository, retry later", "devices", len(failed), "retry", delay.String(), "error", err)
				retry = time.After(delay)
			} else if wasFailing {
				this.config.GetLogger().Info("device repository readable again, all devices handled")
			}
		}
	}()
	return nil
}

// handleDeviceInfoUpdates handles the devices and returns those that have to be retried, with the last
// error, and the enabled devices of previouslyFailed that are registered now; their event values have to
// be refreshed, because the events sent while they were not registered, the initial refresh included,
// were dropped
func (this *Connector) handleDeviceInfoUpdates(devices []model.ZigbeeDeviceInfo, previouslyFailed []model.ZigbeeDeviceInfo) (failed []model.ZigbeeDeviceInfo, recovered []model.ZigbeeDeviceInfo, err error) {
	wasFailing := map[string]bool{}
	for _, device := range previouslyFailed {
		wasFailing[device.IeeeAddress] = true
	}
	for _, device := range devices {
		result := this.handleDeviceInfoUpdate(device)
		if result.Retry {
			failed = append(failed, device)
			err = result.Err
			continue
		}
		if result.Err == nil && wasFailing[device.IeeeAddress] && !device.Disabled {
			recovered = append(recovered, device)
		}
	}
	return failed, recovered, err
}

// refreshEventValues requests the current values of the devices from zigbee2mqtt; in the background,
// because a publish waits for the broker and would hold up the device list handling
func (this *Connector) refreshEventValues(wg *sync.WaitGroup, devices []model.ZigbeeDeviceInfo) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, device := range devices {
			err := this.zigbee.RefreshDeviceEventValues(device)
			if err != nil {
				this.config.GetLogger().Error("unable to refresh event values", "deviceIeeeAddress", device.IeeeAddress, "error", err)
			}
		}
	}()
}

type DeviceInfoUpdateResult struct {
	NewDeviceType string
	Err           error
	// Retry is set if the device type could not be read from the device repository; handling the device
	// again later may succeed
	Retry bool
}

func (this *DeviceInfoUpdateResult) WithErr(err error) *DeviceInfoUpdateResult {
	this.Err = err
	return this
}

func (this *DeviceInfoUpdateResult) WithNewDeviceType(newDeviceType string) *DeviceInfoUpdateResult {
	this.NewDeviceType = newDeviceType
	return this
}

func (this *Connector) handleDeviceInfoUpdate(device model.ZigbeeDeviceInfo) *DeviceInfoUpdateResult {
	result := &DeviceInfoUpdateResult{}
	if isValidDevice(device) {
		deviceId := this.getDeviceId(device)
		deviceName := this.getDeviceName(device)
		deviceTypeId, err := this.getDeviceTypeId(device)
		if errors.Is(err, model.NoMatchingDeviceTypeFound) {
			this.config.GetLogger().Warn("unable to find matching device type", "deviceIeeeAddress", device.IeeeAddress, "error", err)
			if this.config.CreateMissingDeviceTypes {
				this.config.GetLogger().Info("create device type", "deviceIeeeAddress", device.IeeeAddress, "error", err)
				deviceTypeId, err = this.createDeviceType(device)
				result.WithNewDeviceType(deviceTypeId)
				if err != nil {
					this.config.GetLogger().Error("unable to create device type", "error", err)
					return result.WithErr(err)
				}
				//if no error: continue with mgw device state publish
			} else {
				missingDtMsg := this.getMissingDeviceTypeMessage(device)
				this.mgw.SendClientError(missingDtMsg)
				return result.WithErr(err)
			}
		} else if err != nil {
			// logged once per device list by the caller; a stopped device repository fails every device
			this.config.GetLogger().Debug("unable to get device type", "deviceIeeeAddress", device.IeeeAddress, "error", err)
			result.Retry = true
			return result.WithErr(err)
		}
		deviceState := this.getDeviceState(device)
		this.storeDeviceState(device.IeeeAddress, deviceState)
		err = this.mgw.SetDevice(deviceId, mgw.DeviceInfo{
			Name:       deviceName,
			State:      deviceState,
			DeviceType: deviceTypeId,
			Attributes: this.deviceAttributes,
		})
		if err != nil {
			this.config.GetLogger().Error("unable to set device", "deviceIeeeAddress", device.IeeeAddress, "error", err)
			return result.WithErr(err)
		}
	}
	return result
}

const AttributeLastMessageMaxAge = "last_message_max_age"

// returns the attributes to be set on every device; an attribute is only added if its config value is set
func getDeviceAttributes(config configuration.Config) (result []mgw.Attribute, err error) {
	for _, attr := range []mgw.Attribute{
		{Key: AttributeLastMessageMaxAge, Value: strings.TrimSpace(config.LastMessageMaxAge)},
	} {
		if attr.Value == "" {
			continue
		}
		_, err = time.ParseDuration(attr.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid duration for device attribute %v: %w", attr.Key, err)
		}
		result = append(result, attr)
	}
	return result, nil
}

func isValidDevice(device model.ZigbeeDeviceInfo) bool {
	if !device.Supported {
		return false
	}
	if !device.InterviewCompleted {
		return false
	}
	if device.Definition.Vendor == "" {
		return false
	}
	if device.Definition.Model == "" {
		return false
	}
	if len(device.Definition.Exposes) == 0 {
		return false
	}
	return true
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

func (this *Connector) getDeviceTypeId(device model.ZigbeeDeviceInfo) (dtId string, err error) {
	var known bool
	dtId, known, err = this.devicerepo.GetKnownDeviceDeviceTypeId(this.getDeviceId(device))
	if err != nil {
		return "", err
	}
	if known {
		return dtId, nil
	}
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
	dt, _, err := this.devicerepo.CreateDeviceTypeWithDistinctAttributes(
		this.zigbeeDeviceToDeviceType(device),
		[]string{
			devicerepo.AttributeUsedForZigbee,
			devicerepo.AttributeZigbeeVendor,
			devicerepo.AttributeZigbeeModel,
		})
	return dt.Id, err
}

func (this *Connector) zigbeeDeviceToDeviceType(device model.ZigbeeDeviceInfo) (dt models.DeviceType) {
	output := this.zigbee.GetZigbeeMessageOutputStruct(device)
	input := this.zigbee.GetZigbeeMessageInputStruct(device)
	vendor := device.Definition.Vendor
	m := device.Definition.Model
	dt = models.DeviceType{
		Name:          "UNFINISHED zigbee2mqtt " + vendor + " " + m,
		DeviceClassId: this.config.CreateMissingDeviceTypesWithDeviceClass,
		Attributes: []models.Attribute{
			{Key: devicerepo.AttributeUsedForZigbee, Value: "true"},
			{Key: devicerepo.AttributeZigbeeVendor, Value: vendor},
			{Key: devicerepo.AttributeZigbeeModel, Value: m},
		},
	}
	if len(input) > 0 {
		dt.Services = append(dt.Services, models.Service{
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
		})
	}
	if len(output) > 0 {
		dt.Services = append(dt.Services, models.Service{
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
		})
	}
	return dt
}

func valueToContentVariable(name string, data interface{}) (result models.ContentVariable) {
	result = models.ContentVariable{
		Name:                name,
		IsVoid:              false,
		SubContentVariables: nil,
		OmitEmpty:           true,
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
