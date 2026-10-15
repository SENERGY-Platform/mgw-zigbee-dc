/*
 * Copyright (c) 2026 InfAI (CC SES)
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
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
)

func TestGetDeviceAttributes(t *testing.T) {
	t.Run("none set", func(t *testing.T) {
		result, err := getDeviceAttributes(configuration.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected no attributes, got %#v", result)
		}
	})
	t.Run("last_message_max_age", func(t *testing.T) {
		result, err := getDeviceAttributes(configuration.Config{LastMessageMaxAge: " 1h "})
		if err != nil {
			t.Fatal(err)
		}
		expected := []mgw.Attribute{{Key: "last_message_max_age", Value: "1h"}}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("\n%#v\n%#v", expected, result)
		}
	})
	t.Run("invalid duration", func(t *testing.T) {
		_, err := getDeviceAttributes(configuration.Config{LastMessageMaxAge: "7d"})
		if err == nil || !strings.Contains(err.Error(), "last_message_max_age") {
			t.Errorf("expected error naming last_message_max_age, got %v", err)
		}
	})
}

func TestDeviceInfoAttributesOmittedIfEmpty(t *testing.T) {
	msg, err := json.Marshal(mgw.DeviceInfo{Name: "n", State: mgw.Online, DeviceType: "dt"})
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != `{"name":"n","state":"online","device_type":"dt"}` {
		t.Error(string(msg))
	}
	msg, err = json.Marshal(mgw.DeviceInfo{Name: "n", State: mgw.Online, DeviceType: "dt", Attributes: []mgw.Attribute{{Key: "k", Value: "v"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != `{"name":"n","state":"online","device_type":"dt","attributes":[{"key":"k","value":"v"}]}` {
		t.Error(string(msg))
	}
}

func TestHandleDeviceInfoUpdateSendsAttributes(t *testing.T) {
	config := configuration.Config{DeviceIdPrefix: "zigbee:", LastMessageMaxAge: "1h"}
	attributes, err := getDeviceAttributes(config)
	if err != nil {
		t.Fatal(err)
	}
	mgwClient := &deviceInfoRecorder{}
	c := Connector{
		config:           config,
		devicestate:      map[string]mgw.State{},
		mgw:              mgwClient,
		devicerepo:       knownDeviceTypeRepo{dtId: "dt1"},
		deviceAttributes: attributes,
	}
	device := resources.DeviceInfoExample[1]
	result := c.handleDeviceInfoUpdate(device)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	info, ok := mgwClient.set["zigbee:"+device.IeeeAddress]
	if !ok {
		t.Fatalf("device not set: %#v", mgwClient.set)
	}
	if info.DeviceType != "dt1" {
		t.Errorf("unexpected device type %v", info.DeviceType)
	}
	expected := []mgw.Attribute{{Key: "last_message_max_age", Value: "1h"}}
	if !reflect.DeepEqual(info.Attributes, expected) {
		t.Errorf("\n%#v\n%#v", expected, info.Attributes)
	}
}

type deviceInfoRecorder struct {
	MgwClient
	mux sync.Mutex
	set map[string]mgw.DeviceInfo
}

func (this *deviceInfoRecorder) SetDevice(deviceId string, deviceInfo mgw.DeviceInfo) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	if this.set == nil {
		this.set = map[string]mgw.DeviceInfo{}
	}
	this.set[deviceId] = deviceInfo
	return nil
}

type knownDeviceTypeRepo struct {
	DeviceRepo
	dtId string
}

func (this knownDeviceTypeRepo) GetKnownDeviceDeviceTypeId(string) (string, bool, error) {
	return this.dtId, true, nil
}

func (this knownDeviceTypeRepo) FindDeviceTypeId(model.ZigbeeDeviceInfo) (string, error) {
	return this.dtId, nil
}
