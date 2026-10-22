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
	"encoding/json"
	"log/slog"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	paho "github.com/eclipse/paho.mqtt.golang"
)

func (this *Client) startDeviceInfoListener() error {
	token := this.mqtt.Subscribe(this.getDeviceInfoTopic(), byte(this.config.ZigbeeQos), this.deviceInfoHandler)
	token.Wait()
	return token.Error()
}

func (this *Client) getDeviceInfoTopic() string {
	return this.config.ZigbeeMqttTopicPrefix + "bridge/devices"
}

func (this *Client) deviceInfoHandler(client paho.Client, message paho.Message) {
	devices, err := parseDeviceInfoMessage(message.Payload(), this.config.GetLogger())
	if err != nil {
		this.config.GetLogger().Error("unable to handle device info message", "error", err)
		return
	}

	doInitialEventRefresh := this.register.Set(devices)

	this.connector.DeviceInfoUpdate(devices)

	if doInitialEventRefresh {
		go func() {
			time.Sleep(200 * time.Millisecond) //ensure resulting events can be handled because device info is processed
			err = this.RefreshEventValues()
			if err != nil {
				this.config.GetLogger().Error("unable to refresh event values", "error", err)
				return
			}
		}()
	}
}

// parseDeviceInfoMessage skips and logs devices whose description can not be read,
// so that one unknown expose format does not block every other device
func parseDeviceInfoMessage(payload []byte, logger *slog.Logger) (devices []model.ZigbeeDeviceInfo, err error) {
	raw := []json.RawMessage{}
	err = json.Unmarshal(payload, &raw)
	if err != nil {
		return nil, err
	}
	devices = []model.ZigbeeDeviceInfo{}
	for i, e := range raw {
		device := model.ZigbeeDeviceInfo{}
		err = json.Unmarshal(e, &device)
		if err != nil {
			logger.Error("skip unreadable device info", "index", i, "deviceIeeeAddress", device.IeeeAddress, "error", err)
			continue
		}
		devices = append(devices, device)
	}
	return devices, nil
}

// GetDeviceList returns a list of model.ZigbeeDeviceInfo
// ctx may be nil (defaults to context with 1 minute timeout)
// if no device is found err == model.ErrNotFound
// if the ctx is done before the register received its first Set() call, GetDeviceList() returns the ctx error
// except if the ctx error is context.Canceled, then no error is returned
func (this *Client) GetDeviceList(ctx context.Context) ([]model.ZigbeeDeviceInfo, error) {
	return this.register.List(ctx)
}

func (this *Client) GetZigbeeMessageOutputStruct(device model.ZigbeeDeviceInfo) (req map[string]interface{}) {
	return GetZigbeeMessageStruct(device, ZigbeeEventAccess)
}

func (this *Client) GetZigbeeMessageInputStruct(device model.ZigbeeDeviceInfo) (req map[string]interface{}) {
	return GetZigbeeMessageStruct(device, ZigbeeSetAccess)
}

// GetZigbeeMessageStruct returns an example message structure for a device, matching the given access
// ref https://www.zigbee2mqtt.io/guide/usage/exposes.html
func GetZigbeeMessageStruct(device model.ZigbeeDeviceInfo, access ZigbeeAccess) (req map[string]interface{}) {
	return GetZigbeeMessageStructOfFeatureList(device.Definition.Exposes, access)
}

// GetZigbeeMessageStructOfFeatureList returns an example message structure of a given exposed feature list, matching the given access
// ref https://www.zigbee2mqtt.io/guide/usage/exposes.html
func GetZigbeeMessageStructOfFeatureList(features []model.ZigbeeDeviceInfoFeature, access ZigbeeAccess) (req map[string]interface{}) {
	req = map[string]interface{}{}
	todo := []model.ZigbeeDeviceInfoFeature{}
	todo = append(todo, features...)
	for i := 0; i < len(todo); i++ {
		field := todo[i]
		if len(field.Features) > 0 {
			//type is "composite" or a specific type like "light"
			sub := GetZigbeeMessageStructOfFeatureList(field.Features, access)
			if len(sub) > 0 || checkZigbeeAccess(field.Access, access) {
				if field.Property != "" {
					if existing, ok := req[field.Property]; ok {
						//merge composite fields if multiple features with the same property name exist
						m, cast := existing.(map[string]interface{})
						if cast {
							for k, v := range sub {
								m[k] = v
							}
							sub = m
						}
					}
					req[field.Property] = sub
				} else {
					for k, v := range sub {
						req[k] = v
					}
				}
			}
		} else if checkZigbeeAccess(field.Access, access) {
			//generic types except composite
			switch field.Type {
			case "numeric":
				if field.ValueMax > 0 {
					req[field.Property] = field.ValueMax
				} else {
					req[field.Property] = 42
				}
			case "binary":
				req[field.Property] = field.ValueOn
			case "enum":
				if len(field.Values) > 0 {
					req[field.Property] = field.Values[0]
				} else {
					req[field.Property] = nil
				}
			case "text":
				req[field.Property] = "text"
			case "list":
				if field.ItemType == nil {
					req[field.Property] = []interface{}{}
					break
				}
				sub := GetZigbeeMessageStructOfFeatureList([]model.ZigbeeDeviceInfoFeature{*field.ItemType}, access)
				if len(field.ItemType.Features) > 0 {
					req[field.Property] = []interface{}{sub}
				} else {
					//zigbee2mqtt removes the property of item_type, so a non-composite item is found under the empty key
					req[field.Property] = []interface{}{sub[field.ItemType.Property]}
				}
			}
		}
	}
	return
}

// ZigbeeAccess encodes https://www.zigbee2mqtt.io/guide/usage/exposes.html#access
type ZigbeeAccess int

const ZigbeeEventAccess ZigbeeAccess = 0b001
const ZigbeeSetAccess ZigbeeAccess = 0b010
const ZigbeeGetAccess ZigbeeAccess = 0b100

func checkZigbeeAccess(access int, checkFor ZigbeeAccess) bool {
	return access&int(checkFor) != 0
}
