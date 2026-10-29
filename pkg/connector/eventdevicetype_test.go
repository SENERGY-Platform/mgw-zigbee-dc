/*
 * Copyright 2025 InfAI (CC SES)
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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/models/go/models"
)

// eventDeviceTypeRepo knows devices by id and matches device types by vendor and model, as the
// device repository does
type eventDeviceTypeRepo struct {
	DeviceRepo
	knownDevices map[string]string
	knownErr     error
	deviceTypes  []model.DeviceType
}

func (this eventDeviceTypeRepo) GetKnownDeviceDeviceTypeId(deviceId string) (string, bool, error) {
	if this.knownErr != nil {
		return "", false, this.knownErr
	}
	dtId, known := this.knownDevices[deviceId]
	return dtId, known, nil
}

func (this eventDeviceTypeRepo) GetDeviceType(dtId string) (model.DeviceType, error) {
	for _, dt := range this.deviceTypes {
		if dt.Id == dtId {
			return dt, nil
		}
	}
	return model.DeviceType{}, fmt.Errorf("%w: id=%v", model.NoMatchingDeviceTypeFound, dtId)
}

func (this eventDeviceTypeRepo) FindDeviceType(device model.ZigbeeDeviceInfo) (model.DeviceType, error) {
	for _, dt := range this.deviceTypes {
		if dt.Name == device.Definition.Vendor+" "+device.Definition.Model {
			return dt, nil
		}
	}
	return model.DeviceType{}, fmt.Errorf("%w: vendor=%v model=%v", model.NoMatchingDeviceTypeFound, device.Definition.Vendor, device.Definition.Model)
}

func TestGetEventServiceIds(t *testing.T) {
	presence := model.DeviceType{Id: "dt-presence", Name: "Tuya ZG-204ZL", Services: []model.Service{{
		LocalId:    "presence",
		Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "contains(value, 'presence')"}},
	}}}
	// zigbee2mqtt 2.14.2 reports the vendor of this sensor as HOBEIAN instead of Tuya
	renamed := model.ZigbeeDeviceInfo{IeeeAddress: "0x01", Definition: model.ZigbeeDeviceInfoDefinition{Vendor: "HOBEIAN", Model: "ZG-204ZL"}}
	unchanged := model.ZigbeeDeviceInfo{IeeeAddress: "0x02", Definition: model.ZigbeeDeviceInfoDefinition{Vendor: "Tuya", Model: "ZG-204ZL"}}
	payload := []byte(`{"presence":true}`)

	newConnector := func(repo eventDeviceTypeRepo) *Connector {
		return &Connector{config: configuration.Config{DeviceIdPrefix: "zigbee:"}, devicerepo: repo}
	}

	t.Run("known device with renamed vendor", func(t *testing.T) {
		c := newConnector(eventDeviceTypeRepo{knownDevices: map[string]string{"zigbee:0x01": "dt-presence"}, deviceTypes: []model.DeviceType{presence}})
		result, err := c.getServiceIds(EventDesc{Device: renamed, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, []string{"get", "presence"}) {
			t.Error(result)
		}
	})

	t.Run("unknown device matched by vendor and model", func(t *testing.T) {
		c := newConnector(eventDeviceTypeRepo{knownDevices: map[string]string{}, deviceTypes: []model.DeviceType{presence}})
		result, err := c.getServiceIds(EventDesc{Device: unchanged, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, []string{"get", "presence"}) {
			t.Error(result)
		}
	})

	t.Run("unknown device with renamed vendor", func(t *testing.T) {
		c := newConnector(eventDeviceTypeRepo{knownDevices: map[string]string{}, deviceTypes: []model.DeviceType{presence}})
		_, err := c.getServiceIds(EventDesc{Device: renamed, Payload: payload})
		if !errors.Is(err, model.NoMatchingDeviceTypeFound) {
			t.Error(err)
		}
	})

	t.Run("known device whose type is not a zigbee device type", func(t *testing.T) {
		c := newConnector(eventDeviceTypeRepo{knownDevices: map[string]string{"zigbee:0x02": "dt-other"}, deviceTypes: []model.DeviceType{presence}})
		result, err := c.getServiceIds(EventDesc{Device: unchanged, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, []string{"get", "presence"}) {
			t.Error(result)
		}
	})

	t.Run("repository not readable", func(t *testing.T) {
		c := newConnector(eventDeviceTypeRepo{knownErr: errors.New("connection refused"), deviceTypes: []model.DeviceType{presence}})
		result, err := c.getServiceIds(EventDesc{Device: unchanged, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, []string{"get", "presence"}) {
			t.Error(result)
		}
	})
}
